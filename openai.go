package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/maruel/genai"
	"github.com/maruel/genai/adapters"
	"github.com/maruel/genai/providers/openaichat"
)

type OpenAIClient struct {
	aiRequestChannel  chan AIRequest
	aiResponseChannel chan AIResponse
	client            *openaichat.Client
	conversations     map[string][]genai.Message
	log               *Logger
}

func NewOpenAIClient(logger Logger, ctx context.Context, apiKey string, aiRequestChannel chan AIRequest, aiResponseChannel chan AIResponse) (*OpenAIClient, error) {
	opts := genai.ProviderOptions{
		Model: genai.ModelCheap,
	}
	c, err := openaichat.New(ctx, &opts, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create genai client: %v", err)
	}
	return &OpenAIClient{
		aiRequestChannel:  aiRequestChannel,
		aiResponseChannel: aiResponseChannel,
		client:            c,
		conversations:     make(map[string][]genai.Message),
		log:               &logger,
	}, nil

}

func (c *OpenAIClient) Run(ctx context.Context, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	for {
		select {
		case <-ctx.Done():
			c.log.Info().Msg("Exiting AI client")
			return
		case req := <-c.aiRequestChannel:
			err := c.continueConversation(ctx, req)
			if err != nil {
				c.log.Error().Err(err).Msg("Failed to continue conversation")
			}
		}
	}
}

func (c *OpenAIClient) continueConversation(ctx context.Context, req AIRequest) error {
	msgs, ok := c.conversations[req.RoomID.String()]
	if !ok {
		msgs = genai.Messages{
			c.startConversation(ctx, req),
		}
	} else {
		msgs = append(msgs, genai.NewTextMessage(fmt.Sprintf("(%s) user: %s\nbot: ", req.Timestamp.String(), req.Message)))
	}

	c.log.Debug().Msg("Generating response...")
	opts := genai.OptionsTools{
		Tools: []genai.ToolDef{
			{
				Name:        "followup_timer",
				Description: "This should be used to indicate that the bot should follow up with the user in the future to check on task progress.",
				Callback: func(ctx2 context.Context, input *FollowupTimerInput) (string, error) {
					return c.followupTimer(ctx2, req, input)
				},
			}, {
				Name:        "switch_task",
				Description: "Any time the user indicates they change tasks this must be called to update the record of what tasks are being done.",
				Callback: func(ctx2 context.Context, input *SwitchTaskInput) (string, error) {
					return c.switchTask(ctx2, req, input)
				},
			},
		},
	}

	res, _, err := adapters.GenSyncWithToolCallLoop(ctx, c.client, msgs, &opts)
	if err != nil {
		return fmt.Errorf("Failed to continue conversation: %v", err)
	}

	for _, m := range res {
		msgs = append(msgs, m)
		// Empty responses are tool call related.
		if m.String() == "" {
		} else {
			c.log.Info().Str("room", req.RoomID.String()).Msg(m.String())
			var toSay string = m.String()
			toSay = strings.Replace(toSay, "bot: ", "", 1)
			c.aiResponseChannel <- AIResponse{
				Message: toSay,
				RoomID:  req.RoomID,
			}
		}
	}
	c.conversations[req.RoomID.String()] = msgs

	return nil
}

type FollowupTimerInput struct {
	Delay time.Duration `json:"delay"`
}

func (c *OpenAIClient) followupTimer(ctx context.Context, req AIRequest, input *FollowupTimerInput) (string, error) {
	c.log.Info().Str("room", req.RoomID.String()).Str("delay", input.Delay.String()).Msg("Followup timer fired.")
	c.aiResponseChannel <- AIResponse{
		Message: fmt.Sprintf("/me ⌛ followup scheduled '%s'", input.Delay.String()),
		RoomID:  req.RoomID,
	}
	return fmt.Sprintf("Followup timer set for %s in the future", input.Delay.String()), nil
}

type SwitchTaskInput struct {
	TaskName string `json:"task_name"`
}

func (c *OpenAIClient) switchTask(ctx context.Context, req AIRequest, input *SwitchTaskInput) (string, error) {
	c.log.Info().Str("room", req.RoomID.String()).Str("task", input.TaskName).Msg("Task Switched")
	c.aiResponseChannel <- AIResponse{
		Message: fmt.Sprintf("/me 📋 notes task '%s'", input.TaskName),
		RoomID:  req.RoomID,
	}

	return fmt.Sprintf("Recorded a switch to task %s at %s", input.TaskName, time.Now().String()), nil
}

/*
func (c *OpenAIClient) handleToolCalls(ctx context.Context, req AIRequest, toolCalls []openai.ChatCompletionMessageToolCallUnion) ([]openai.ChatCompletionMessageParamUnion, error) {
	var results []openai.ChatCompletionMessageParamUnion
	for _, toolCall := range toolCalls {
		if toolCall.Function.Name == "followup_timer" {
			// Extract the description from the function call arguments
			var args map[string]interface{}
			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				return results, fmt.Errorf("Failed to unmarshal tool call JSON: %v", err)
			}
			interval := args["interval"].(string)
			c.log.Info().Str("sender", req.Sender).Str("interval", interval).Msg("Followup scheduled")
			c.aiResponseChannel <- AIResponse{
				Message:
				RoomID:  req.RoomID,
			}
			results = append(results, openai.ToolMessage("followup scheduled", toolCall.ID))
		} else if toolCall.Function.Name == "switch_task" {
			// Extract the description from the function call arguments
			var args map[string]interface{}
			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				return results, fmt.Errorf("Failed to unmarshal toll call JSON: %v", err)
			}
			description := args["description"].(string)
			var started string
			if args["started"] == nil {
				started = ""
			} else {
				started = args["started"].(string)
			}
			c.log.Info().Str("sender", req.Sender).Str("description", description).Str("started", started).Msg("User switched tasks")
			c.aiResponseChannel <- AIResponse{
				Message:
				RoomID:  req.RoomID,
			}
			results = append(results, openai.ToolMessage("task updated", toolCall.ID))
		} else {
			c.log.Warn().Str("function", toolCall.Function.Name).Msg("Unrecognized tool call")
		}
	}
	return results, nil
}
*/

func (c *OpenAIClient) startConversation(ctx context.Context, req AIRequest) genai.Message {
	return genai.NewTextMessage(fmt.Sprintf("This is a text chat conversation between an employee and a chatbot helping to manage timecards. Messages from the user will start with '(timestamp) user:'. Messages from the bot will start with 'bot:'. Sometimes the user won't say anything for a long time and the chatbot needs to follow-up with them. When time passes, there will be a prompt like '<20 minutes pass>'. The bot should then prompt the user to provide a bit of information about what they've been working on during that time. The bot should be interested to know what the user's goals are at a high level and should pay attention to any difficulties or frustrations the user experiences.\n\n(%s) user: %s\nbot:", req.Timestamp.String(), req.Message))
}

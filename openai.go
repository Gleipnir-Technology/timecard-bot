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
					return c.followupSchedule(ctx2, req, input)
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
	DelayInSeconds int64 `json:"delay_in_seconds"`
}

func (c *OpenAIClient) followupFire(ctx context.Context, req AIRequest, duration time.Duration) {
	if err := ctx.Err(); err != nil {
		c.log.Info().Str("room", req.RoomID.String()).Msg("Context canceled")
		return
	}
	msgs, ok := c.conversations[req.RoomID.String()]
	if !ok {
		c.log.Warn().Str("room", req.RoomID.String()).Msg("No messages for room")
		return
	}
	msgs = append(msgs, genai.NewTextMessage(fmt.Sprintf("<%s time passed>", duration.String())))
	res, err := c.client.GenSync(ctx, msgs)
	if err != nil {
		c.log.Error().Str("room", req.RoomID.String()).Err(err).Msg("Failed to continue after timer")
		return
	}
	msgs = append(msgs, res.Message)
	var toSay string = res.String()
	toSay = strings.Replace(toSay, "bot: ", "", 1)
	c.aiResponseChannel <- AIResponse{
		Message: toSay,
		RoomID:  req.RoomID,
	}
	c.conversations[req.RoomID.String()] = msgs
}

func (c *OpenAIClient) followupSchedule(ctx context.Context, req AIRequest, input *FollowupTimerInput) (string, error) {
	c.log.Info().Str("room", req.RoomID.String()).Int64("delay", input.DelayInSeconds).Msg("Followup timer fired.")
	duration, err := time.ParseDuration(fmt.Sprintf("%ds", input.DelayInSeconds))
	if err != nil {
		return "", fmt.Errorf("Failed to parse %d as a valid duration: %v", input.DelayInSeconds, err)
	}
	c.aiResponseChannel <- AIResponse{
		Message: fmt.Sprintf("⌛ followup scheduled '%s'", duration.String()),
		RoomID:  req.RoomID,
	}
	time.AfterFunc(duration, func() {
		c.followupFire(ctx, req, duration)
	})
	return fmt.Sprintf("Followup timer set for %s in the future", duration.String()), nil
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

func (c *OpenAIClient) startConversation(ctx context.Context, req AIRequest) genai.Message {
	return genai.NewTextMessage(fmt.Sprintf("This is a text chat conversation between an employee and a chatbot helping to manage timecards. Messages from the user will start with '(timestamp) user:'. Messages from the bot will start with 'bot:'. Sometimes the user won't say anything for a long time and the chatbot needs to follow-up with them. When time passes, there will be a prompt like '<20 minutes pass>'. The bot should then prompt the user to provide a bit of information about what they've been working on during that time. The bot should be interested to know what the user's goals are at a high level and should pay attention to any difficulties or frustrations the user experiences.\n\n(%s) user: %s\nbot:", req.Timestamp.String(), req.Message))
}

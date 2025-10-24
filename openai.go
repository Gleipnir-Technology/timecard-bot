package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	//"github.com/openai/openai-go/v3/shared"
)

type OpenAIClient struct {
	aiRequestChannel  chan AIRequest
	aiResponseChannel chan AIResponse
	client            *openai.Client
	conversations     map[string]openai.ChatCompletionNewParams
	log               *Logger
}

func NewOpenAIClient(logger Logger, apiKey string, aiRequestChannel chan AIRequest, aiResponseChannel chan AIResponse) (*OpenAIClient, error) {
	openAIClient := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithDebugLog(LoggerShim(logger)),
	)
	return &OpenAIClient{
		aiRequestChannel:  aiRequestChannel,
		aiResponseChannel: aiResponseChannel,
		client:            &openAIClient,
		conversations:     make(map[string]openai.ChatCompletionNewParams),
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
	param, ok := c.conversations[req.RoomID.String()]
	if !ok {
		c.startConversation(ctx, req)
		param = c.conversations[req.RoomID.String()]
	}
	param.Messages = append(param.Messages, openai.UserMessage(fmt.Sprintf("(%s) user: %s\n", req.Timestamp.String(), req.Message)))
	completion, err := c.client.Chat.Completions.New(ctx, param)
	if err != nil {
		return fmt.Errorf("Failed to get completion: %v", err)
	}
	toolMessages, err := c.handleToolCalls(ctx, req, completion.Choices[0].Message.ToolCalls)
	if err != nil {
		return fmt.Errorf("Failed to handle tool calls: %v", err)
	}
	if len(toolMessages) > 0 {
		param.Messages = append(param.Messages, completion.Choices[0].Message.ToParam())
		for _, msg := range toolMessages {
			param.Messages = append(param.Messages, msg)
		}
		c.log.Debug().Int("count", len(toolMessages)).Msg("Responding to tool request")
		completion, err = c.client.Chat.Completions.New(ctx, param)
		if err != nil {
			return fmt.Errorf("Failed to handle tool calls: %v", err)
		}
		param.Messages = append(param.Messages, completion.Choices[0].Message.ToParam())
		//completion, err = c.client.Chat.Completions.New(ctx, param)
		//if err != nil {
		//return fmt.Errorf("Failed to get ongoing completion: %v", err)
		//}
	} else {
		c.log.Debug().Msg("Continuing conversation without tool responses")
		//param.Messages = append(param.Messages, completion.Choices[0].Message.ToParam())
	}
	var toSay string = completion.Choices[0].Message.Content
	toSay = strings.Replace(toSay, "bot: ", "", 1)
	c.aiResponseChannel <- AIResponse{
		Message: toSay,
		RoomID:  req.RoomID,
	}
	c.conversations[req.RoomID.String()] = param
	return nil
}

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
				Message: fmt.Sprintf("/me ⌛ followup scheduled '%s'", interval),
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
				Message: fmt.Sprintf("/me 📋 notes task '%s'", description),
				RoomID:  req.RoomID,
			}
			results = append(results, openai.ToolMessage("task updated", toolCall.ID))
		} else {
			c.log.Warn().Str("function", toolCall.Function.Name).Msg("Unrecognized tool call")
		}
	}
	return results, nil
}

func (c *OpenAIClient) startConversation(ctx context.Context, req AIRequest) {
	param := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("This is a text chat conversation between an employee and a chatbot helping to manage timecards. Messages from the user will start with '(timestamp) user:'. Messages from the bot will start with 'bot:'. Sometimes the user won't say anything for a long time and the chatbot needs to follow-up with them. When time passes, there will be a prompt like '<20 minutes pass>'. The bot should then prompt the user to provide a bit of information about what they've been working on during that time. The bot should be interested to know what the user's goals are at a high level and should pay attention to any difficulties or frustrations the user experiences.\n\n"),
		},
		Model: openai.ChatModelO4Mini,
		Seed:  openai.Int(1),
		Tools: []openai.ChatCompletionToolUnionParam{{
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Function: openai.FunctionDefinitionParam{
					Name:        "switch_task",
					Description: openai.String("Any time the user indicates they change tasks this must be called to update the record of what tasks are being done."),
					Parameters: openai.FunctionParameters{
						"type": "object",
						"properties": map[string]interface{}{
							"description": map[string]string{
								"type": "string",
							},
							"started": map[string]string{
								"format": "date-time",
								"type":   "string",
							},
						},
						"required": []string{"description"},
					},
				},
			},
		}, {
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Function: openai.FunctionDefinitionParam{
					Name:        "followup_timer",
					Description: openai.String("This should be used to indicate that the bot should follow up with the user in the future to check on task progress."),
					Parameters: openai.FunctionParameters{
						"type": "object",
						"properties": map[string]interface{}{
							"description": map[string]string{
								"type": "string",
							},
							"interval": map[string]string{
								"format": "time",
								"type":   "string",
							},
						},
						"required": []string{"interval"},
					},
				},
			},
		}},
	}

	c.log.Info().Str("sender", req.Sender).Msg("Starting conversation")
	c.conversations[req.RoomID.String()] = param
}

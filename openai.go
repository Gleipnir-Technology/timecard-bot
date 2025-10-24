package main

import (
	"context"
	"encoding/json"
	"fmt"
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
		//option.WithDebugLog(LoggerShim(logger)),
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
		return c.startConversation(ctx, req)
	}
	param.Messages = append(param.Messages, openai.UserMessage(fmt.Sprintf("user: %s\n", req.Message)))
	completion, err := c.client.Chat.Completions.New(ctx, param)
	if err != nil {
		return fmt.Errorf("Failed to get ongoing completion: %v", err)
	}
	c.aiResponseChannel <- AIResponse{
		Message: completion.Choices[0].Message.Content,
		RoomID:  req.RoomID,
	}
	param.Messages = append(param.Messages, completion.Choices[0].Message.ToParam())
	c.conversations[req.RoomID.String()] = param
	return nil
}

func (c *OpenAIClient) handleToolCalls(ctx context.Context, req AIRequest, toolCalls []openai.ChatCompletionMessageToolCallUnion) ([]openai.ChatCompletionMessageParamUnion, error) {
	var results []openai.ChatCompletionMessageParamUnion
	for _, toolCall := range toolCalls {
		if toolCall.Function.Name == "switch_task" {
			// Extract the description from the function call arguments
			var args map[string]interface{}
			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				return results, fmt.Errorf("Failed to unmarshal toll call JSON: %v", err)
			}
			description := args["description"].(string)
			started := args["started"].(string)
			c.log.Info().Str("sender", req.Sender).Str("description", description).Str("started", started).Msg("User switched tasks")
			results = append(results, openai.ToolMessage("task updated", toolCall.ID))
		} else {
			c.log.Warn().Str("function", toolCall.Function.Name).Msg("Unrecognized tool call")
		}
	}
	return results, nil
}

func (c *OpenAIClient) startConversation(ctx context.Context, req AIRequest) error {
	param := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(fmt.Sprintf("This is a text chat conversation between an employee and a chatbot helping to manage timecards. Messages from the user will start with 'user:'. Sometimes the user won't say anything for a long time and the chatbot needs to follow-up with them. When time passes, there will be a prompt like '<20 minutes pass>'. The bot should then prompt the user to provide a bit of information about what they've been working on during that time. The bot should be interested to know what the user's goals are at a high level and should pay attention to any difficulties or frustrations the user experiences.\n\nuser: %s\n", req.Message)),
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
		}},
	}

	c.log.Info().Str("sender", req.Sender).Msg("Starting conversation")
	completion, err := c.client.Chat.Completions.New(ctx, param)
	if err != nil {
		return fmt.Errorf("Failed to get completion: %v", err)
	}
	param.Messages = append(param.Messages, completion.Choices[0].Message.ToParam())
	toolMessages, err := c.handleToolCalls(ctx, req, completion.Choices[0].Message.ToolCalls)
	if err != nil {
		return fmt.Errorf("Failed to handle tool calls: %v", err)
	}
	c.log.Debug().Int("count", len(toolMessages)).Msg("Got tool response messages")
	if len(toolMessages) > 0 {
		for _, msg := range toolMessages {
			param.Messages = append(param.Messages, msg)
		}
		completion, err = c.client.Chat.Completions.New(ctx, param)
		if err != nil {
			return fmt.Errorf("Failed to handle tool calls: %v", err)
		}
	}
	c.aiResponseChannel <- AIResponse{
		Message: completion.Choices[0].Message.Content,
		RoomID:  req.RoomID,
	}

	c.conversations[req.RoomID.String()] = param
	return nil
}

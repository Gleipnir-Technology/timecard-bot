package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
}

func NewOpenAIClient(logger *log.Logger, apiKey string, aiRequestChannel chan AIRequest, aiResponseChannel chan AIResponse) (*OpenAIClient, error) {
	openAIClient := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithDebugLog(logger),
	)
	return &OpenAIClient{
		aiRequestChannel:  aiRequestChannel,
		aiResponseChannel: aiResponseChannel,
		client:            &openAIClient,
		conversations:     make(map[string]openai.ChatCompletionNewParams),
	}, nil
}

func (c *OpenAIClient) Run(ctx context.Context, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	for {
		select {
		case <-ctx.Done():
			log.Println("Exiting AI client")
			return
		case req := <-c.aiRequestChannel:
			err := c.continueConversation(ctx, req)
			if err != nil {
				log.Printf("Failure! %v", err)
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

func (c *OpenAIClient) handleToolCalls(ctx context.Context, req AIRequest, toolCalls []openai.ChatCompletionMessageToolCallUnion) error {
	for _, toolCall := range toolCalls {
		if toolCall.Function.Name == "switch_taks" {
			// Extract the description from the function call arguments
			var args map[string]interface{}
			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				return fmt.Errorf("Failed to unmarshal toll call JSON: %v", err)
			}
			description := args["description"].(string)
			started := args["started"].(string)
			log.Printf("User %s Switched task to %s (%s)", req.Sender, description, started)
		}
	}
	return nil
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
					Description: openai.String("Indicate that the user switched tasks and save a description of the new task"),
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

	log.Println("Starting conversation with %s", req.Sender)
	completion, err := c.client.Chat.Completions.New(ctx, param)
	if err != nil {
		return fmt.Errorf("Failed to get completion: %v", err)
	}
	c.aiResponseChannel <- AIResponse{
		Message: completion.Choices[0].Message.Content,
		RoomID:  req.RoomID,
	}
	param.Messages = append(param.Messages, completion.Choices[0].Message.ToParam())
	err = c.handleToolCalls(ctx, req, completion.Choices[0].Message.ToolCalls)
	if err != nil {
		return fmt.Errorf("Failed to handle tool calls: %v", err)
	}
	c.conversations[req.RoomID.String()] = param
	return nil
}

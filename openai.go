package main

import (
	"context"
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

func NewOpenAIClient(apiKey string, aiRequestChannel chan AIRequest, aiResponseChannel chan AIResponse) (*OpenAIClient, error) {
	openAIClient := openai.NewClient(
		option.WithAPIKey(apiKey),
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
			c.continueConversation(ctx, req)
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
	/*
		chatCompletion, err := openAIClient.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Say this is a test"),
			},
			Model: openai.ChatModelO4Mini,
		})
		if err != nil {
			panic(err.Error())
		}
	*/
}

func (c *OpenAIClient) startConversation(ctx context.Context, req AIRequest) error {
	param := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(fmt.Sprintf("This is a text chat conversation between an employee and a chatbot helping to manage timecards. Messages from the user will start with 'user:'. Sometimes the user won't say anything for a long time and the chatbot needs to follow-up with them. When time passes, there will be a prompt like '<20 minutes pass>'. The bot should then prompt the user to provide a bit of information about what they've been working on during that time. The bot should be interested to know what the user's goals are at a high level and should pay attention to any difficulties or frustrations the user experiences.\n\nuser: %s\n", req.Message)),
		},
		Seed:  openai.Int(1),
		Model: openai.ChatModelO4Mini,
	}

	completion, err := c.client.Chat.Completions.New(ctx, param)
	if err != nil {
		return fmt.Errorf("Failed to get completion: %v", err)
	}
	c.aiResponseChannel <- AIResponse{
		Message: completion.Choices[0].Message.Content,
		RoomID:  req.RoomID,
	}
	param.Messages = append(param.Messages, completion.Choices[0].Message.ToParam())
	c.conversations[req.RoomID.String()] = param
	return nil
}

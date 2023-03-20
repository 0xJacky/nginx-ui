package test

import (
	"context"
	"fmt"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/pkg/errors"
	"github.com/sashabaranov/go-openai"
	"io"
	"os"
	"testing"
)

func TestChatGPT(t *testing.T) {
	settings.Init("../../app.ini")
	c := openai.NewClient(settings.OpenAISettings.Token)

	ctx := context.Background()

	req := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo0301,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "帮我写一个 nginx 配置文件的示例",
			},
		},
		Stream: true,
	}
	stream, err := c.CreateChatCompletionStream(ctx, req)
	if err != nil {
		fmt.Printf("CompletionStream error: %v\n", err)
		return
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return
		}

		if err != nil {
			fmt.Printf("Stream error: %v\n", err)
			return
		}

		fmt.Printf("%v", response.Choices[0].Delta.Content)
		_ = os.Stdout.Sync()
	}
}

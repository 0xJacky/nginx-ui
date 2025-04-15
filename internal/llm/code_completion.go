package llm

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/sashabaranov/go-openai"
	"github.com/uozi-tech/cosy/logger"
)

const (
	MaxTokens   = 100
	Temperature = 1
	// Build system prompt and user prompt
	SystemPrompt = "You are a code completion assistant. " +
		"Complete the provided code snippet based on the context and instruction." +
		"[IMPORTANT] Keep the original code indentation."
)

// Position the cursor position
type Position struct {
	Row    int `json:"row"`
	Column int `json:"column"`
}

// CodeCompletionRequest the code completion request
type CodeCompletionRequest struct {
	RequestID string   `json:"request_id"`
	UserID    uint64   `json:"user_id"`
	Context   string   `json:"context"`
	Code      string   `json:"code"`
	Suffix    string   `json:"suffix"`
	Language  string   `json:"language"`
	Position  Position `json:"position"`
}

var (
	requestContext = make(map[uint64]context.CancelFunc)
	mutex          sync.Mutex
)

func (c *CodeCompletionRequest) Send() (completedCode string, err error) {
	if cancel, ok := requestContext[c.UserID]; ok {
		logger.Infof("Code completion request cancelled for user %d", c.UserID)
		cancel()
	}

	mutex.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requestContext[c.UserID] = cancel
	mutex.Unlock()
	defer func() {
		mutex.Lock()
		delete(requestContext, c.UserID)
		mutex.Unlock()
	}()

	openaiClient, err := GetClient()
	if err != nil {
		return
	}

	// Build user prompt with code and instruction
	userPrompt := "Here is a file written in " + c.Language + ":\n```\n" + c.Context + "\n```\n"
	userPrompt += "I'm editing at row " + strconv.Itoa(c.Position.Row) + ", column " + strconv.Itoa(c.Position.Column) + ".\n"
	userPrompt += "Code before cursor:\n```\n" + c.Code + "\n```\n"

	if c.Suffix != "" {
		userPrompt += "Code after cursor:\n```\n" + c.Suffix + "\n```\n"
	}

	userPrompt += "Instruction: Only provide the completed code that should be inserted at the cursor position without explanations. " +
		"The code should be syntactically correct and follow best practices for " + c.Language + "."

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: SystemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: userPrompt,
		},
	}

	req := openai.ChatCompletionRequest{
		Model:       settings.OpenAISettings.GetCodeCompletionModel(),
		Messages:    messages,
		MaxTokens:   MaxTokens,
		Temperature: Temperature,
	}

	// Make a direct (non-streaming) call to the API
	response, err := openaiClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return
	}

	completedCode = response.Choices[0].Message.Content
	// extract the last word of the code
	lastWord := extractLastWord(c.Code)
	completedCode = cleanCompletionResponse(completedCode, lastWord)
	logger.Infof("Code completion response: %s", completedCode)
	return
}

// extractLastWord extract the last word of the code
func extractLastWord(code string) string {
	if code == "" {
		return ""
	}

	// define a regex to match word characters (letters, numbers, underscores)
	re := regexp.MustCompile(`[a-zA-Z0-9_]+$`)

	// find the last word of the code
	match := re.FindString(code)

	return match
}

// cleanCompletionResponse removes any <think></think> tags and their content from the completion response
// and strips the already entered code from the completion
func cleanCompletionResponse(response string, lastWord string) (cleanResp string) {
	// remove <think></think> tags and their content using regex
	re := regexp.MustCompile(`<think>[\s\S]*?</think>`)

	cleanResp = re.ReplaceAllString(response, "")

	// remove markdown code block tags
	codeBlockRegex := regexp.MustCompile("```(?:[a-zA-Z]+)?\n((?:.|\n)*?)\n```")
	matches := codeBlockRegex.FindStringSubmatch(cleanResp)

	if len(matches) > 1 {
		// extract the code block content
		cleanResp = strings.TrimSpace(matches[1])
	} else {
		// if no code block is found, keep the original response
		cleanResp = strings.TrimSpace(cleanResp)
	}

	// remove markdown backticks
	cleanResp = strings.Trim(cleanResp, "`")

	// if there is a last word, and the completion result starts with the last word, remove the already entered part
	if lastWord != "" && strings.HasPrefix(cleanResp, lastWord) {
		cleanResp = cleanResp[len(lastWord):]
	}

	return
}

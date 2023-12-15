package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/sashabaranov/go-openai"
)

type ChatGPTCompletionMessages []openai.ChatCompletionMessage

// Scan value into Jsonb, implements sql.Scanner interface
func (j *ChatGPTCompletionMessages) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}

	result := make([]openai.ChatCompletionMessage, 0)
	err := json.Unmarshal(bytes, &result)
	*j = result

	return err
}

// Value return json value, implement driver.Valuer interface
func (j *ChatGPTCompletionMessages) Value() (driver.Value, error) {
	return json.Marshal(*j)
}

type ChatGPTLog struct {
	Name    string                    `json:"name"`
	Content ChatGPTCompletionMessages `json:"content" gorm:"serializer:json"`
}

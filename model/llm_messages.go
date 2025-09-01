package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

type LLMCompletionMessages []openai.ChatCompletionMessage

// Scan value into Jsonb, implements sql.Scanner interface
func (j *LLMCompletionMessages) Scan(value interface{}) error {
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
func (j *LLMCompletionMessages) Value() (driver.Value, error) {
	return json.Marshal(*j)
}

type LLMMessages struct {
	Name    string                `json:"name"`
	Content LLMCompletionMessages `json:"content" gorm:"serializer:json"`
}

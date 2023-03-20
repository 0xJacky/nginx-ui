package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/sashabaranov/go-openai"
)

type JSON []openai.ChatCompletionMessage

// Scan scan value into Jsonb, implements sql.Scanner interface
func (j *JSON) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}

	var result []openai.ChatCompletionMessage
	err := json.Unmarshal(bytes, &result)
	*j = result
	return err
}

// Value return json value, implement driver.Valuer interface
func (j *JSON) Value() (driver.Value, error) {
	return json.Marshal(*j)
}

type ChatGPTLog struct {
	Name    string `json:"name"`
	Content JSON   `json:"content" gorm:"serializer:json"`
}

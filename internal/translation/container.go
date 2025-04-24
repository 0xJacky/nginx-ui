package translation

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Container contains a source string and a map of arguments.
type Container struct {
	Message string         `json:"message"`
	Args    map[string]any `json:"args,omitempty"`
}

// C creates a new Container.
func C(message string, args ...map[string]any) *Container {
	if len(args) == 0 {
		return &Container{
			Message: message,
		}
	}
	return &Container{
		Message: message,
		Args:    args[0],
	}
}

// ToString returns the source string with the arguments replaced.
func (c *Container) ToString() (result string) {
	result = c.Message
	for k, v := range c.Args {
		result = strings.ReplaceAll(result, "%{"+k+"}", fmt.Sprintf("%v", v))
	}
	return
}

// ToJSON returns the arguments as a JSON object.
func (c *Container) ToJSON() (result []byte, err error) {
	return json.Marshal(c)
}

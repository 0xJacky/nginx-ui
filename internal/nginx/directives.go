package nginx

import (
	_ "embed"
	"encoding/json"
)

//go:embed nginx_directives.json
var directivesJson []byte

type Directive struct {
	Links []string `json:"links"`
}

func GetDirectives() (map[string]Directive, error) {
	var directives map[string]Directive
	err := json.Unmarshal(directivesJson, &directives)
	if err != nil {
		return nil, err
	}
	return directives, nil
}

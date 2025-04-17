//go:generate go run . -project . -type ts -output ../../app/src/constants/errors -ignore-dirs .devcontainer,app,.github
package main

import "github.com/uozi-tech/cosy/errdef"

func main() {
	errdef.Generate()
}

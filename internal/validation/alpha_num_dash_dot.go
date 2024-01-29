package validation

import (
	val "github.com/go-playground/validator/v10"
	"regexp"
)

func alphaNumDashDot(fl val.FieldLevel) bool {
	return regexp.MustCompile(`^[a-zA-Z0-9-.]+$`).MatchString(fl.Field().String())
}

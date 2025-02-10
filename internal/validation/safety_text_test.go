package validation

import (
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_safetyText(t *testing.T) {
	v := validator.New()

	err := v.RegisterValidation("safety_test", safetyText)

	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, v.Var("Home", "safety_test"))
	assert.Nil(t, v.Var("本地", "safety_test"))
	assert.Nil(t, v.Var("桜 です", "safety_test"))
	assert.Nil(t, v.Var("st-weqmnvme.enjdur_", "safety_test"))
	assert.Nil(t, v.Var("4412272A-7E63-4C3C-BAFB-EA78F66A0437", "safety_test"))
	assert.Nil(t, v.Var("gpt-4o", "safety_test"))
	assert.Nil(t, v.Var("gpt-3.5", "safety_test"))
	assert.Nil(t, v.Var("gpt-4-turbo-1106", "safety_test"))
	assert.Error(t, v.Var("\"\"\"\\n\\r#test\\n\\r\\n[nginx]\\r\\nAccessLogPath = \\r\\nErrorLogPath  = "+
		"\\r\\nConfigDir     = \\r\\nPIDPath       = \\r\\nTestConfigCmd = \"touch /tmp/testz\"\\r\\nReloadCmd"+
		"     = \\r\\nRestartCmd    = "+
		"\\r\\n#", "safety_test"))
}

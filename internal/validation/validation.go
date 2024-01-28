package validation

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/gin-gonic/gin/binding"
	val "github.com/go-playground/validator/v10"
)

func Init() {
	v, ok := binding.Validator.Engine().(*val.Validate)
	if !ok {
		logger.Fatal("binding validator engine is not initialized")
	}

	err := v.RegisterValidation("alphanumdash", alphaNumDash)

	if err != nil {
		logger.Fatal(err)
	}

	err = v.RegisterValidation("publickey", isPublicKey)

	if err != nil {
		logger.Fatal(err)
	}

	err = v.RegisterValidation("privatekey", isPrivateKey)

	if err != nil {
		logger.Fatal(err)
	}

	err = v.RegisterValidation("publickey_path", isPublicKeyPath)

	if err != nil {
		logger.Fatal(err)
	}

	err = v.RegisterValidation("privatekey_path", isPrivateKeyPath)

	if err != nil {
		logger.Fatal(err)
	}

	return
}

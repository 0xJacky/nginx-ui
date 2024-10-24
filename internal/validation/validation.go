package validation

import (
	"github.com/gin-gonic/gin/binding"
	val "github.com/go-playground/validator/v10"
	"github.com/uozi-tech/cosy/logger"
)

func Init() {
	v, ok := binding.Validator.Engine().(*val.Validate)
	if !ok {
		logger.Fatal("failed to initialize binding validator engine")
	}

	err := v.RegisterValidation("safety_text", safetyText)
	if err != nil {
		logger.Fatal(err)
	}

	err = v.RegisterValidation("certificate", isCertificate)
	if err != nil {
		logger.Fatal(err)
	}

	err = v.RegisterValidation("privatekey", isPrivateKey)
	if err != nil {
		logger.Fatal(err)
	}

	err = v.RegisterValidation("certificate_path", isCertificatePath)
	if err != nil {
		logger.Fatal(err)
	}

	err = v.RegisterValidation("privatekey_path", isPrivateKeyPath)
	if err != nil {
		logger.Fatal(err)
	}

	err = v.RegisterValidation("auto_cert_key_type", autoCertKeyType)
	if err != nil {
		logger.Fatal(err)
	}

	return
}

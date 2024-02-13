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

	err := v.RegisterValidation("alpha_num_dash_dot", alphaNumDashDot)

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

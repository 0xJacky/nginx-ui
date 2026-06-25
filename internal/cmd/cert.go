package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/migrate"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/uozi-tech/cosy"
	sqlite "github.com/uozi-tech/cosy-driver-sqlite"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
	"github.com/urfave/cli/v3"
)

var CertCommand = &cli.Command{
	Name:  "cert",
	Usage: "Manage certificates",
	Commands: []*cli.Command{
		{
			Name:   "import",
			Usage:  "Import an existing certificate from explicit paths",
			Action: ImportCertificate,
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "name", Usage: "certificate name"},
				&cli.StringFlag{Name: "cert", Usage: "path to the certificate file"},
				&cli.StringFlag{Name: "key", Usage: "path to the private key file"},
				&cli.StringFlag{Name: "key-type", Usage: "optional private key type override"},
			},
		},
		{
			Name:   "scan",
			Usage:  "Recursively scan the Nginx SSL directory and import certificate/key pairs",
			Action: ScanCertificates,
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "new-only", Usage: "only import certificates whose path, name and fingerprint are not already in the database"},
				&cli.StringFlag{Name: "name-prefix", Usage: "optional prefix for imported certificate names"},
				&cli.StringFlag{Name: "key-type", Usage: "optional private key type override"},
			},
		},
	},
}

func ImportCertificate(_ context.Context, command *cli.Command) error {
	if err := initCertCommand(command); err != nil {
		return err
	}

	opts, err := importOptionsFromCommand(command)
	if err != nil {
		return err
	}

	certModel, err := cert.ImportExistingCertificate(opts)
	if err != nil {
		return err
	}

	fmt.Printf("imported certificate %q\n", certModel.Name)
	fmt.Printf("certificate: %s\n", certModel.SSLCertificatePath)
	fmt.Printf("private key: %s\n", certModel.SSLCertificateKeyPath)
	fmt.Printf("key type: %s\n", certModel.KeyType)
	return nil
}

func ScanCertificates(_ context.Context, command *cli.Command) error {
	if err := initCertCommand(command); err != nil {
		return err
	}

	keyType := certcrypto.KeyType(command.String("key-type"))
	if keyType != "" && !helper.IsValidKeyType(keyType) {
		return cert.NewInvalidKeyTypeError(string(keyType))
	}

	namePrefix := command.String("name-prefix")
	newOnly := command.Bool("new-only")
	pairs := make([]cert.DiscoveredCertificatePair, 0)
	skipped, failed := 0, 0

	results, err := cert.ScanCertificateSSLDirectoryResults()
	if err != nil {
		return err
	}

	for _, result := range results {
		if result.Error != nil {
			skipped++
			fmt.Printf("skipped %s: %s\n", result.Dir, result.Reason)
			continue
		}
		if result.Pair != nil {
			pairs = append(pairs, *result.Pair)
		}
	}

	for i := range pairs {
		pairs[i].Name = namePrefix + pairs[i].Name
	}
	if newOnly {
		filtered, err := cert.FilterNewCertificatePairs(pairs)
		if err != nil {
			return err
		}
		skipped += len(pairs) - len(filtered)
		pairs = filtered
	}

	imported := 0
	for _, pair := range pairs {
		certModel, err := cert.ImportExistingCertificate(cert.ImportCertificateOptions{
			Name:     pair.Name,
			CertPath: pair.SSLCertificatePath,
			KeyPath:  pair.SSLCertificateKeyPath,
			KeyType:  keyType,
		})
		if err != nil {
			failed++
			fmt.Printf("failed %s: %s\n", pair.Dir, err)
			continue
		}

		imported++
		fmt.Printf("imported %s as %q\n", pair.Dir, certModel.Name)
	}

	fmt.Printf("summary: imported=%d skipped=%d failed=%d\n", imported, skipped, failed)
	return nil
}

func importOptionsFromCommand(command *cli.Command) (cert.ImportCertificateOptions, error) {
	certPath := command.String("cert")
	keyPath := command.String("key")
	keyType := certcrypto.KeyType(command.String("key-type"))

	if keyType != "" && !helper.IsValidKeyType(keyType) {
		return cert.ImportCertificateOptions{}, cert.NewInvalidKeyTypeError(string(keyType))
	}
	if certPath == "" || keyPath == "" {
		return cert.ImportCertificateOptions{}, cert.ErrCertificatePathsRequired
	}

	return cert.ImportCertificateOptions{
		Name:     command.String("name"),
		CertPath: certPath,
		KeyPath:  keyPath,
		KeyType:  keyType,
	}, nil
}

func initCertCommand(command *cli.Command) error {
	confPath := command.String("config")
	settings.Init(confPath)

	if cSettings.ServerSettings.RunMode == "" {
		cSettings.ServerSettings.RunMode = gin.ReleaseMode
	}
	gin.SetMode(cSettings.ServerSettings.RunMode)
	logger.Init(cSettings.ServerSettings.RunMode)

	cosy.RegisterMigrationsBeforeAutoMigrate(migrate.BeforeAutoMigrate)
	cosy.RegisterModels(model.GenerateAllModel()...)
	cosy.RegisterMigration(migrate.Migrations)

	db := cosy.InitDB(sqlite.Open(filepath.Dir(cSettings.ConfPath), settings.DatabaseSettings))
	model.Use(db)
	query.Init(db)

	return nil
}

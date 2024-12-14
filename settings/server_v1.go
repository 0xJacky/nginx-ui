package settings

import (
	"github.com/elliotchance/orderedmap/v3"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
	"github.com/uozi-tech/cosy/settings"
	"gopkg.in/ini.v1"
	"os"
	"reflect"
)

// Note: This section will be deprecated in the future version.

type serverV1 struct {
	HttpHost             string   `json:"http_host" protected:"true"`
	HttpPort             string   `json:"http_port" protected:"true"`
	RunMode              string   `json:"run_mode" protected:"true"`
	JwtSecret            string   `json:"jwt_secret" protected:"true"`
	NodeSecret           string   `json:"node_secret" protected:"true"`
	HTTPChallengePort    string   `json:"http_challenge_port"`
	Email                string   `json:"email" protected:"true"`
	Database             string   `json:"database" protected:"true"`
	StartCmd             string   `json:"start_cmd" protected:"true"`
	CADir                string   `json:"ca_dir"`
	Demo                 bool     `json:"demo" protected:"true"`
	PageSize             int      `json:"page_size" protected:"true"`
	GithubProxy          string   `json:"github_proxy"`
	CertRenewalInterval  int      `json:"cert_renewal_interval"`
	RecursiveNameservers []string `json:"recursive_nameservers"`
	SkipInstallation     bool     `json:"skip_installation" protected:"true"`
	InsecureSkipVerify   bool     `json:"insecure_skip_verify" protected:"true"`
	Name                 string   `json:"name"`
}

type settingsV2 struct {
	// Cosy
	App      settings.App
	Server   settings.Server
	DataBase Database
	// Nginx UI
	Auth      Auth
	Casdoor   Casdoor
	Cert      Cert
	Cluster   Cluster
	Crypto    Crypto
	Http      HTTP
	Logrotate Logrotate
	Nginx     Nginx
	Node      Node
	OpenAI    OpenAI
	Terminal  Terminal
	WebAuthn  WebAuthn
}

func (v1 *serverV1) migrateToV2() (v2 *settingsV2) {
	v2 = &settingsV2{}
	v2.Server.Host = v1.HttpHost
	v2.Server.Port = cast.ToUint(v1.HttpPort)
	v2.Server.RunMode = v1.RunMode
	v2.App.JwtSecret = v1.JwtSecret
	v2.App.PageSize = v1.PageSize
	v2.Node.Secret = v1.NodeSecret
	v2.Cert.HTTPChallengePort = v1.HTTPChallengePort
	v2.Cert.Email = v1.Email
	v2.DataBase.Name = v1.Database
	v2.Terminal.StartCmd = v1.StartCmd
	v2.Cert.CADir = v1.CADir
	v2.Node.Demo = v1.Demo
	v2.Http.GithubProxy = v1.GithubProxy
	v2.Cert.RenewalInterval = v1.CertRenewalInterval
	v2.Cert.RecursiveNameservers = v1.RecursiveNameservers
	v2.Node.SkipInstallation = v1.SkipInstallation
	v2.Http.InsecureSkipVerify = v1.InsecureSkipVerify
	v2.Node.Name = v1.Name

	if v1.Database == "" {
		v2.DataBase.Name = "database"
	}

	return
}

func isZeroValue(v reflect.Value) bool {
	zeroValue := reflect.Zero(v.Type()).Interface()
	return reflect.DeepEqual(v.Interface(), zeroValue)
}

func mergeStructs(src, dst interface{}) {
	dstVal := reflect.ValueOf(dst).Elem()
	srcVal := reflect.ValueOf(src).Elem()

	for i := 0; i < dstVal.NumField(); i++ {
		dstField := dstVal.Field(i)
		srcField := srcVal.Field(i)
		if isZeroValue(dstField) {
			dstField.Set(srcField)
		}
	}
	return
}

func migrate(confPath string) {
	logger.Init("debug")
	Conf, err := ini.LoadSources(ini.LoadOptions{
		Loose:        true,
		AllowShadows: true,
	}, confPath)
	if err != nil {
		logger.Fatalf("setting.init, fail to parse 'app.ini': %v", err)
	}

	var v1 = &serverV1{}
	err = Conf.Section("server").MapTo(v1)
	if err != nil {
		logger.Error(err)
		return
	}

	// If settings is v1, jwt_secret is not empty.
	if v1.JwtSecret == "" {
		return
	}

	// Cosy
	app := &settings.App{}
	server := &settings.Server{}
	database := &Database{}
	// Nginx UI
	auth := &Auth{}
	casdoor := &Casdoor{}
	cert := &Cert{}
	cluster := &Cluster{}
	crypto := &Crypto{}
	http := &HTTP{}
	logrotate := &Logrotate{}
	nginx := &Nginx{}
	node := &Node{}
	openai := &OpenAI{}
	terminal := &Terminal{}
	webauthn := &WebAuthn{}

	var migrated = orderedmap.NewOrderedMap[string, any]()
	migrated.Set("app", app)
	migrated.Set("server", server)
	migrated.Set("database", database)
	migrated.Set("auth", auth)
	migrated.Set("casdoor", casdoor)
	migrated.Set("cert", cert)
	migrated.Set("cluster", cluster)
	migrated.Set("crypto", crypto)
	migrated.Set("http", http)
	migrated.Set("logrotate", logrotate)
	migrated.Set("nginx", nginx)
	migrated.Set("node", node)
	migrated.Set("openai", openai)
	migrated.Set("terminal", terminal)
	migrated.Set("webauthn", webauthn)

	for name, ptr := range migrated.AllFromFront() {
		err = Conf.Section(name).MapTo(ptr)
		if err != nil {
			logger.Error("migrate.MapTo %s err: %v", name, err)
		}
	}

	v2 := v1.migrateToV2()

	mergeStructs(&v2.App, app)
	mergeStructs(&v2.Server, server)
	mergeStructs(&v2.DataBase, database)
	mergeStructs(&v2.Auth, auth)
	mergeStructs(&v2.Casdoor, casdoor)
	mergeStructs(&v2.Cert, cert)
	mergeStructs(&v2.Cluster, cluster)
	mergeStructs(&v2.Crypto, crypto)
	mergeStructs(&v2.Http, http)
	mergeStructs(&v2.Logrotate, logrotate)
	mergeStructs(&v2.Nginx, nginx)
	mergeStructs(&v2.Node, node)
	mergeStructs(&v2.OpenAI, openai)
	mergeStructs(&v2.Terminal, terminal)
	mergeStructs(&v2.WebAuthn, webauthn)

	Conf = ini.Empty()

	for section, ptr := range migrated.AllFromFront() {
		err = Conf.Section(section).ReflectFrom(ptr)
		if err != nil {
			logger.Fatalf("migrate.ReflectFrom %s err: %v", section, err)
		}
	}

	err = Conf.SaveTo(confPath)
	if err != nil {
		logger.Fatalf("Fail to save the migrated settings: %v", err)
		return
	}

	migrateEnv()
}

func migrateEnv() {
	deprecated := orderedmap.NewOrderedMap[string, string]()
	deprecated.Set("SERVER_HTTP_HOST", "SERVER_HOST")
	deprecated.Set("SERVER_HTTP_PORT", "SERVER_PORT")
	deprecated.Set("SERVER_JWT_SECRET", "APP_JWT_SECRET")
	deprecated.Set("SERVER_NODE_SECRET", "NODE_SECRET")
	deprecated.Set("SERVER_HTTP_CHALLENGE_PORT", "CERT_HTTP_CHALLENGE_PORT")
	deprecated.Set("SERVER_EMAIL", "CERT_EMAIL")
	deprecated.Set("SERVER_DATABASE", "DATABASE_NAME")
	deprecated.Set("SERVER_START_CMD", "TERMINAL_START_CMD")
	deprecated.Set("SERVER_CA_DIR", "CERT_CA_DIR")
	deprecated.Set("SERVER_DEMO", "NODE_DEMO")
	deprecated.Set("SERVER_PAGE_SIZE", "APP_PAGE_SIZE")
	deprecated.Set("SERVER_GITHUB_PROXY", "HTTP_GITHUB_PROXY")
	deprecated.Set("SERVER_CERT_RENEWAL_INTERVAL", "CERT_RENEWAL_INTERVAL")
	deprecated.Set("SERVER_RECURSIVE_NAMESERVERS", "CERT_RECURSIVE_NAMESERVERS")
	deprecated.Set("SERVER_SKIP_INSTALLATION", "NODE_SKIP_INSTALLATION")
	deprecated.Set("SERVER_NAME", "NODE_NAME")

	for d, n := range deprecated.AllFromFront() {
		oldValue := os.Getenv(EnvPrefix + d)
		if oldValue != "" {
			_ = os.Setenv(EnvPrefix+n, oldValue)
			logger.Warnf("The environment variable %s is deprecated and has been automatically migrated to %s. "+
				"Please update your environment variables as automatic migration may be removed in the future.",
				EnvPrefix+d, EnvPrefix+n)
		}
	}
}

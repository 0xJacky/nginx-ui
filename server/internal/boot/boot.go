package boot

import (
	"github.com/0xJacky/Nginx-UI/server/internal/analytic"
	"github.com/0xJacky/Nginx-UI/server/internal/cert"
	"github.com/0xJacky/Nginx-UI/server/internal/logger"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/query"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/go-co-op/gocron"
	"github.com/google/uuid"
	"mime"
	"runtime"
	"time"
)

func Kernel() {
	defer recovery()

	async := []func(){
		InitJsExtensionType,
		InitDatabase,
		InitNodeSecret,
	}

	syncs := []func(){
		analytic.RecordServerAnalytic,
	}

	for _, v := range async {
		v()
	}

	for _, v := range syncs {
		go v()
	}
}

func InitAfterDatabase() {
	syncs := []func(){
		InitAutoObtainCert,
		analytic.RetrieveNodesStatus,
	}

	for _, v := range syncs {
		go v()
	}
}

func recovery() {
	if err := recover(); err != nil {
		buf := make([]byte, 1024)
		runtime.Stack(buf, false)
		logger.Errorf("%s\n%s", err, buf)
	}
}

func InitDatabase() {
	if "" != settings.ServerSettings.JwtSecret {
		db := model.Init()
		query.Init(db)

		InitAfterDatabase()
	}
}

func InitNodeSecret() {
	if "" == settings.ServerSettings.NodeSecret {
		logger.Warn("NodeSecret is empty, generating...")
		settings.ServerSettings.NodeSecret = uuid.New().String()
		settings.ReflectFrom()

		err := settings.Save()
		if err != nil {
			logger.Error("Error save settings")
		}
		logger.Warn("Generated NodeSecret: ", settings.ServerSettings.NodeSecret)
	}
}

func InitJsExtensionType() {
	// Hack: fix wrong Content Type of .js file on some OS platforms
	// See https://github.com/golang/go/issues/32350
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")
}

func InitAutoObtainCert() {
	s := gocron.NewScheduler(time.UTC)
	job, err := s.Every(30).Minute().SingletonMode().Do(cert.AutoObtain)

	if err != nil {
		logger.Fatalf("AutoCert Job: %v, Err: %v\n", job, err)
	}

	s.StartAsync()
}

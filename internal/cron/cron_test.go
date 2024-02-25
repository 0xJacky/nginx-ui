package cron

import (
	"github.com/0xJacky/Nginx-UI/internal/kernal"
	"github.com/0xJacky/Nginx-UI/settings"
	"testing"
	"time"
)

func TestRestartLogrotate(t *testing.T) {
	settings.Init("../../app.ini")

	kernal.InitDatabase()

	InitCronJobs()

	time.Sleep(5 * time.Second)

	RestartLogrotate()

	time.Sleep(2 * time.Second)
}

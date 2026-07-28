package cert

import (
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
)

func TestBuildExpiryNotificationUsesShortLivedThresholds(t *testing.T) {
	notBefore := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	info := &Info{NotBefore: notBefore, NotAfter: notBefore.Add(160 * time.Hour)}
	certModel := &model.Cert{Name: "192.0.2.1", Domains: []string{"192.0.2.1"}}

	tests := []struct {
		name      string
		remaining time.Duration
		stage     expiryNotificationStage
		nType     model.NotificationType
	}{
		{name: "healthy certificate has no notice", remaining: 41 * time.Hour},
		{name: "quarter lifetime remaining is notice", remaining: 40 * time.Hour,
			stage: expiryStageNotice, nType: model.NotificationInfo},
		{name: "one eighth lifetime remaining is urgent", remaining: 20 * time.Hour,
			stage: expiryStageUrgent, nType: model.NotificationWarning},
		{name: "six hours remaining is critical", remaining: 6 * time.Hour,
			stage: expiryStageCritical, nType: model.NotificationError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := info.NotAfter.Add(-tt.remaining)
			notice := buildExpiryNotification(certModel, info, now)
			if tt.stage == "" {
				if notice != nil {
					t.Fatalf("notice = %+v, want nil", notice)
				}
				return
			}
			if notice == nil {
				t.Fatal("notice = nil")
			}
			if notice.Stage != tt.stage || notice.Type != tt.nType {
				t.Fatalf("notice = %+v, want stage %q and type %d", notice, tt.stage, tt.nType)
			}
		})
	}
}

func TestBuildExpiryNotificationEscalatesOncePerCertificate(t *testing.T) {
	notBefore := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	notAfter := notBefore.Add(160 * time.Hour)
	info := &Info{NotBefore: notBefore, NotAfter: notAfter}
	certModel := &model.Cert{
		Name:                     "192.0.2.1",
		LastExpiryNotifyNotAfter: &notAfter,
		LastExpiryNotifyStage:    string(expiryStageNotice),
	}

	if notice := buildExpiryNotification(certModel, info, notAfter.Add(-39*time.Hour)); notice != nil {
		t.Fatalf("duplicate notice = %+v, want nil", notice)
	}

	notice := buildExpiryNotification(certModel, info, notAfter.Add(-20*time.Hour))
	if notice == nil || notice.Stage != expiryStageUrgent {
		t.Fatalf("escalated notice = %+v, want urgent", notice)
	}

	newNotAfter := notAfter.Add(160 * time.Hour)
	newInfo := &Info{NotBefore: notAfter, NotAfter: newNotAfter}
	notice = buildExpiryNotification(certModel, newInfo, newNotAfter.Add(-40*time.Hour))
	if notice == nil || notice.Stage != expiryStageNotice {
		t.Fatalf("renewed certificate notice = %+v, want notice", notice)
	}
}

func TestBuildExpiryNotificationWaitsForARIRenewalTime(t *testing.T) {
	notBefore := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	info := &Info{NotBefore: notBefore, NotAfter: notBefore.Add(160 * time.Hour)}
	now := info.NotAfter.Add(-20 * time.Hour)
	renewAt := now.Add(time.Hour)
	certModel := &model.Cert{
		Name:            "192.0.2.1",
		NextAutoRenewAt: &renewAt,
	}

	if notice := buildExpiryNotification(certModel, info, now); notice != nil {
		t.Fatalf("notice before ARI renewal time = %+v, want nil", notice)
	}
}

func TestBuildExpiryNotificationUsesOrderedStandardThresholds(t *testing.T) {
	notBefore := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	info := &Info{NotBefore: notBefore, NotAfter: notBefore.Add(90 * 24 * time.Hour)}
	certModel := &model.Cert{Name: "example.com"}

	tests := []struct {
		remaining time.Duration
		stage     expiryNotificationStage
	}{
		{remaining: 14 * 24 * time.Hour, stage: expiryStageNotice},
		{remaining: 7 * 24 * time.Hour, stage: expiryStageWarning},
		{remaining: 3 * 24 * time.Hour, stage: expiryStageUrgent},
		{remaining: 24 * time.Hour, stage: expiryStageCritical},
	}

	for _, tt := range tests {
		notice := buildExpiryNotification(certModel, info, info.NotAfter.Add(-tt.remaining))
		if notice == nil || notice.Stage != tt.stage {
			t.Fatalf("remaining %s: notice = %+v, want %q", tt.remaining, notice, tt.stage)
		}
	}
}

package nodeauth

import (
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
)

func TestCredentialRotationDue(t *testing.T) {
	now := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		credential *model.NodeCredential
		expected   bool
	}{
		{
			name: "active credential younger than rotation age",
			credential: &model.NodeCredential{
				Model:  model.Model{CreatedAt: now.Add(-automaticCredentialRotationAge + time.Hour)},
				Status: model.NodeCredentialStatusActive,
			},
		},
		{
			name: "active credential at rotation age",
			credential: &model.NodeCredential{
				Model:  model.Model{CreatedAt: now.Add(-automaticCredentialRotationAge)},
				Status: model.NodeCredentialStatusActive,
			},
			expected: true,
		},
		{
			name: "rotated timestamp supersedes creation timestamp",
			credential: &model.NodeCredential{
				Model:     model.Model{CreatedAt: now.Add(-2 * automaticCredentialRotationAge)},
				Status:    model.NodeCredentialStatusActive,
				RotatedAt: timePointer(now.Add(-time.Hour)),
			},
		},
		{
			name: "unfinished rotation is retried",
			credential: &model.NodeCredential{
				Status: model.NodeCredentialStatusRotating,
			},
			expected: true,
		},
		{
			name: "revoked credential is never rotated",
			credential: &model.NodeCredential{
				Status:    model.NodeCredentialStatusActive,
				RevokedAt: timePointer(now.Add(-time.Hour)),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, CredentialRotationDue(test.credential, now))
		})
	}
}

func timePointer(value time.Time) *time.Time {
	return &value
}

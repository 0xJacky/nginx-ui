package ingress

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
)

func TestComputePhase(t *testing.T) {
	applied := model.IngressCondition{Type: model.IngressConditionConfigApplied, Status: true, Reason: "Applied"}
	cert := func(status bool, reason string) model.IngressCondition {
		return model.IngressCondition{Type: model.IngressConditionCertReady, Status: status, Reason: reason}
	}
	cases := []struct {
		name       string
		generation int64
		conditions []model.IngressCondition
		want       string
	}{
		{name: "never applied", generation: 1, want: model.IngressPhasePending},
		{name: "failed apply wins over a pending generation", generation: 2,
			conditions: []model.IngressCondition{{Type: model.IngressConditionConfigApplied, Reason: "TestFailed"}},
			want:       model.IngressPhaseError},
		{name: "pending generation", generation: 2, conditions: []model.IngressCondition{applied},
			want: model.IngressPhasePending},
		{name: "issued", generation: 1, conditions: []model.IngressCondition{applied, cert(true, CertReasonIssued)},
			want: model.IngressPhaseReady},
		{name: "existing", generation: 1, conditions: []model.IngressCondition{applied, cert(true, CertReasonExisting)},
			want: model.IngressPhaseReady},
		{name: "not required", generation: 1, conditions: []model.IngressCondition{applied, cert(true, CertReasonNotRequired)},
			want: model.IngressPhaseReady},
		{name: "served while a reissue runs", generation: 1, conditions: []model.IngressCondition{applied, cert(true, CertReasonIssuing)},
			want: model.IngressPhaseDegraded},
		{name: "served after a failed reissue", generation: 1, conditions: []model.IngressCondition{applied, cert(true, CertReasonIssueFailed)},
			want: model.IngressPhaseDegraded},
		{name: "no certificate", generation: 1, conditions: []model.IngressCondition{applied, cert(false, CertReasonDNSNotReady)},
			want: model.IngressPhaseDegraded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ing := &model.Ingress{Generation: tc.generation, Status: model.IngressStatus{ObservedGeneration: 1}}
			for _, c := range tc.conditions {
				SetCondition(&ing.Status, c)
			}
			assert.Equal(t, tc.want, ComputePhase(ing))
		})
	}
}

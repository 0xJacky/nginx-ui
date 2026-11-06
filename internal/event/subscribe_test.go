package event

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscribeReceivesPublishedEvents(t *testing.T) {
	var got []Event
	unsubscribe := Subscribe(func(e Event) { got = append(got, e) })

	Publish(Event{Type: TypeCertIssued, Data: "a"})
	assert.Len(t, got, 1)
	assert.Equal(t, TypeCertIssued, got[0].Type)

	unsubscribe()
	Publish(Event{Type: TypeCertIssued, Data: "b"})
	assert.Len(t, got, 1)
}

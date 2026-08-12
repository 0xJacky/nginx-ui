package notification

import (
	"context"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/require"
)

func TestSendTestMessageContextPropagatesCancellation(t *testing.T) {
	const notifierType = "context-security-test"
	RegisterExternalNotifier(notifierType, func(
		ctx context.Context,
		_ *model.ExternalNotify,
		_ *ExternalMessage,
	) error {
		<-ctx.Done()
		return ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := SendTestMessageContext(ctx, notifierType, "en", map[string]string{"url": "https://example.com"})
	require.ErrorIs(t, err, context.Canceled)
}

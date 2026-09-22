package notification

import (
	"context"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
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

func TestSendTestMessageContextSetsGoToURL(t *testing.T) {
	const notifierType = "context-go-to-url-test"
	goToURL := ""
	originalOrigins := settings.WebAuthnSettings.RPOrigins
	settings.WebAuthnSettings.RPOrigins = []string{"https://ui.example.com"}
	t.Cleanup(func() {
		settings.WebAuthnSettings.RPOrigins = originalOrigins
	})

	RegisterExternalNotifier(notifierType, func(
		_ context.Context,
		_ *model.ExternalNotify,
		msg *ExternalMessage,
	) error {
		goToURL = msg.GetGoToURL()
		return nil
	})

	err := SendTestMessageContext(context.Background(), notifierType, "en", map[string]string{"url": "https://example.com"})
	require.NoError(t, err)
	require.Equal(t, "https://ui.example.com/#/preference", goToURL)
}

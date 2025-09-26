package notification

import (
	"context"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

var (
	externalNotifierRegistry      = make(map[string]ExternalNotifierHandlerFunc)
	externalNotifierRegistryMutex = &sync.RWMutex{}
)

type ExternalNotifierHandlerFunc func(ctx context.Context, n *model.ExternalNotify, msg *ExternalMessage) error

func externalNotifierHandler(n *model.ExternalNotify) (ExternalNotifierHandlerFunc, error) {
	externalNotifierRegistryMutex.RLock()
	defer externalNotifierRegistryMutex.RUnlock()
	notifier, ok := externalNotifierRegistry[n.Type]
	if !ok {
		return nil, ErrNotifierNotFound
	}
	return notifier, nil
}

func RegisterExternalNotifier(name string, handler ExternalNotifierHandlerFunc) {
	externalNotifierRegistryMutex.Lock()
	defer externalNotifierRegistryMutex.Unlock()
	externalNotifierRegistry[name] = handler
}

type ExternalMessage struct {
	Notification *model.Notification
}

func (n *ExternalMessage) Send() {
	en := query.ExternalNotify
	externalNotifies, err := en.Where(en.Enabled.Is(true)).Find()
	if err != nil {
		logger.Error(err)
		return
	}
	ctx := context.Background()
	for _, externalNotify := range externalNotifies {
		go func(externalNotify *model.ExternalNotify) {
			notifier, err := externalNotifierHandler(externalNotify)
			if err != nil {
				logger.Error(err)
				return
			}
			notifier(ctx, externalNotify, n)
		}(externalNotify)
	}
}

// SendWithConfig sends the message with direct configuration parameters
func (n *ExternalMessage) SendWithConfig(notifyType, language string, config map[string]string) error {
	// Create a temporary ExternalNotify object with the provided parameters
	externalNotify := &model.ExternalNotify{
		Type:     notifyType,
		Language: language,
		Config:   config,
	}

	ctx := context.Background()
	notifier, err := externalNotifierHandler(externalNotify)
	if err != nil {
		return err
	}

	return notifier(ctx, externalNotify, n)
}

func (n *ExternalMessage) GetTitle(lang string) string {
	if n.Notification == nil {
		return ""
	}

	dict, ok := translation.Dict[lang]
	if !ok {
		dict = translation.Dict["en"]
	}

	title, err := dict.Translate(n.Notification.Title)
	if err != nil {
		return n.Notification.Title
	}

	return title
}

func (n *ExternalMessage) GetContent(lang string) string {
	if n.Notification == nil {
		return ""
	}

	if n.Notification.Details == nil {
		return n.Notification.Content
	}

	dict, ok := translation.Dict[lang]
	if !ok {
		dict = translation.Dict["en"]
	}

	content, err := dict.Translate(n.Notification.Content, n.Notification.Details)
	if err != nil {
		return n.Notification.Content
	}

	return content
}

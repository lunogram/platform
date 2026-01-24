package channels

import (
	"fmt"

	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/lunogram/platform/services/nexus/internal/store"
)

type ComposeOptions struct {
	Devices store.Devices
}

func Compose(channel providers.Channel, config map[string]any, template store.Template, user *store.User, opts *ComposeOptions) (*providers.SendRequest[map[string]any], error) {
	switch channel {
	case providers.ChannelEmail:
		return ComposeEmail(config, template, user)
	case providers.ChannelSMS:
		return ComposeSMS(config, template, user)
	case providers.ChannelPush:
		if opts == nil {
			return nil, fmt.Errorf("push channel requires devices")
		}
		return ComposePush(config, template, user, opts.Devices)
	default:
		return nil, fmt.Errorf("unsupported channel: %s", channel)
	}
}

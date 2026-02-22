package channels

import (
	"fmt"

	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules/providers"
)

type ComposeOptions struct {
	Devices subjects.Devices
}

func Compose(channel providers.Channel, config map[string]any, template management.Template, user *subjects.User, opts *ComposeOptions) (*providers.SendRequest[map[string]any], error) {
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

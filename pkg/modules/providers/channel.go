package providers

// Channel represents a communication channel type.
type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelSMS   Channel = "sms"
	ChannelPush  Channel = "push"
)

// String returns the string representation of the channel.
func (c Channel) String() string {
	return string(c)
}

// IsValid checks if the channel is a valid known channel type.
func (c Channel) IsValid() bool {
	switch c {
	case ChannelEmail, ChannelSMS, ChannelPush:
		return true
	default:
		return false
	}
}

// Platform represents a push notification platform.
type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
	PlatformWeb     Platform = "web"
	PlatformEmail   Platform = "email"
)

// String returns the string representation of the platform.
func (p Platform) String() string {
	return string(p)
}

// IsValid checks if the platform is a valid known platform type.
func (p Platform) IsValid() bool {
	switch p {
	case PlatformIOS, PlatformAndroid, PlatformWeb, PlatformEmail:
		return true
	default:
		return false
	}
}

package providers

// Channel represents a communication channel type.
type Channel string

const (
	ChannelEmail   Channel = "email"
	ChannelSMS     Channel = "sms"
	ChannelPush    Channel = "push"
	ChannelWebhook Channel = "webhook"
)

// String returns the string representation of the channel.
func (c Channel) String() string {
	return string(c)
}

// IsValid checks if the channel is a valid known channel type.
func (c Channel) IsValid() bool {
	switch c {
	case ChannelEmail, ChannelSMS, ChannelPush, ChannelWebhook:
		return true
	default:
		return false
	}
}

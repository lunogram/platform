package consumer

import "github.com/lunogram/platform/internal/pubsub/schemas"

// Namespace prefixes stream names, consumer names, and subject patterns
// to isolate NATS JetStream resources. An empty namespace leaves all
// names unchanged, preserving backward compatibility.
type Namespace string

// Stream returns the stream name with the namespace prefix applied.
// If the namespace is empty, the original name is returned.
func (ns Namespace) Stream(name string) string {
	if ns == "" {
		return name
	}
	return string(ns) + "-" + name
}

// Consumer returns the consumer name with the namespace prefix applied.
// If the namespace is empty, the original name is returned.
func (ns Namespace) Consumer(name string) string {
	if ns == "" {
		return name
	}
	return string(ns) + "-" + name
}

// Subject returns the subject pattern with the namespace prefix applied.
// If the namespace is empty, the original pattern is returned.
// Example: "users.process.>" becomes "ns.users.process.>".
func (ns Namespace) Subject(pattern string) string {
	if ns == "" {
		return pattern
	}
	return string(ns) + "." + pattern
}

// PrefixSubject returns a schemas.Subject with the namespace prefix applied.
// If the namespace is empty, the original subject is returned.
func (ns Namespace) PrefixSubject(subject schemas.Subject) schemas.Subject {
	if ns == "" {
		return subject
	}
	return schemas.Subject(string(ns) + "." + string(subject))
}

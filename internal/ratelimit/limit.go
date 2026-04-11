package ratelimit

import (
	providers "github.com/lunogram/platform/pkg/modules/providers"
)

// Limit is an alias for providers.Limit so that existing internal consumers
// continue to compile without changes.
type Limit = providers.Limit

// ProviderKey is re-exported from the providers package.
var ProviderKey = providers.ProviderKey

// NewLimit is re-exported from the providers package.
var NewLimit = providers.NewLimit

// NewLimitWithKey is re-exported from the providers package.
var NewLimitWithKey = providers.NewLimitWithKey

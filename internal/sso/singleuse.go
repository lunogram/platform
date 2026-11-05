package sso

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// singleUse is the Redis mechanics the OpenID Connect and SAML flow stores
// share: a value written under a key for a TTL, and read back exactly once.
//
// Single use is why the read is GETDEL rather than GET followed by DEL. A
// replayed authorization response -- from a browser back button, a leaked
// referrer, or somebody who captured the callback URL -- must not be able to
// open a second session. Both protocols need that property, and a second
// implementation of it would be a second chance to get it wrong.
type singleUse struct {
	client *goredis.Client
	prefix string
	ttl    time.Duration
}

func (s *singleUse) save(ctx context.Context, key string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.prefix+key, encoded, s.ttl).Err()
}

// consume reads a value and deletes it in the same round trip. A key that was
// never issued, has expired, or has already been redeemed is reported as
// [ErrFlowNotFound] and the three are deliberately indistinguishable.
func (s *singleUse) consume(ctx context.Context, key string, into any) error {
	if key == "" {
		return ErrFlowNotFound
	}

	encoded, err := s.client.GetDel(ctx, s.prefix+key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return ErrFlowNotFound
	}
	if err != nil {
		return err
	}

	if err := json.Unmarshal(encoded, into); err != nil {
		return ErrFlowNotFound
	}
	return nil
}

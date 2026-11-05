package sso

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// ErrAssertionReplayed reports that an assertion carrying this ID has already
// been accepted by this deployment.
var ErrAssertionReplayed = errors.New("sso: this assertion has already been used")

// SAMLFlow is the half of an authentication request that never leaves the
// server. Its OpenID Connect twin is [Flow]; the fields differ because the
// protocols prove different things.
//
// There is no nonce and no PKCE verifier here. SAML binds a response to the
// request that asked for it with InResponseTo, which the identity provider
// copies from the AuthnRequest ID and signs along with everything else, so the
// value that has to survive the round trip is the request ID rather than a
// secret of ours.
type SAMLFlow struct {
	// ProviderID is the provider this login was started with. The endpoint
	// refuses a response whose path names a different one: RelayState is a
	// value the browser carries, and without this a flow issued for one
	// provider could be redeemed at another's assertion consumer service and
	// proved against its entity id and certificates.
	ProviderID string `json:"provider_id"`
	// RequestID is the AuthnRequest's ID, which the response must name in
	// InResponseTo. It is what makes the response an answer to a question this
	// deployment actually asked rather than an unsolicited assertion.
	RequestID string `json:"request_id"`
	// Binding is the flow's half of the browser binding. Its twin is set on the
	// browser that started the login, as a cookie, and the endpoint refuses a
	// response the two do not agree on.
	Binding string `json:"binding"`
	// Redirect is where the console is sent once the session exists. It is
	// validated as a same-site path before it is stored, never on the way out.
	Redirect string `json:"redirect"`
}

// SAMLFlowStore holds outstanding authentication requests in Redis, keyed by
// the RelayState the browser carries.
//
// Redis rather than process memory for the same reason as [FlowStore]: a login
// starts on whichever replica answered /start and finishes on whichever answers
// the assertion consumer service, and those are not the same process.
type SAMLFlowStore struct {
	single singleUse
}

// NewSAMLFlowStore returns nil when there is no Redis client. See
// [NewFlowStore] for why that is not wrapped in a non-nil store.
func NewSAMLFlowStore(client *goredis.Client, prefix string) *SAMLFlowStore {
	if client == nil {
		return nil
	}
	return &SAMLFlowStore{single: singleUse{client: client, prefix: prefix + "auth:saml:flow:", ttl: FlowTTL}}
}

// Save records a flow under its RelayState for the TTL.
func (s *SAMLFlowStore) Save(ctx context.Context, relayState string, flow SAMLFlow) error {
	return s.single.save(ctx, relayState, flow)
}

// Consume reads a flow and deletes it in the same round trip, so a RelayState
// can be redeemed exactly once.
func (s *SAMLFlowStore) Consume(ctx context.Context, relayState string) (SAMLFlow, error) {
	var flow SAMLFlow
	if err := s.single.consume(ctx, relayState, &flow); err != nil {
		return SAMLFlow{}, err
	}
	return flow, nil
}

// AssertionReplayStore records the assertions this deployment has already
// accepted, so none is accepted twice.
//
// Redeeming the flow is not enough on its own. The flow is keyed by RelayState,
// which the identity provider does not sign; an assertion is the signed part,
// and SAML explicitly requires a service provider to reject one it has seen
// before. Without this, a captured assertion could be replayed under a fresh
// RelayState from a login the attacker starts themselves.
type AssertionReplayStore struct {
	client *goredis.Client
	prefix string
}

// NewAssertionReplayStore returns nil when there is no Redis client, so the
// driver's collaborator check refuses at boot rather than at the first login.
func NewAssertionReplayStore(client *goredis.Client, prefix string) *AssertionReplayStore {
	if client == nil {
		return nil
	}
	return &AssertionReplayStore{client: client, prefix: prefix + "auth:saml:assertion:"}
}

// Claim records an assertion ID and reports [ErrAssertionReplayed] if it was
// already recorded.
//
// until is the moment the assertion stops being valid on its own terms, which
// is what bounds how long the record has to be kept: once the assertion would
// be refused for being expired anyway, remembering it buys nothing. Passing the
// assertion's own expiry rather than a fixed window is what keeps this store
// from growing without limit on a busy deployment.
//
// SETNX is what makes this a claim rather than a check followed by a write. Two
// replicas handed the same assertion at the same moment must not both find it
// absent and both let it through.
func (s *AssertionReplayStore) Claim(ctx context.Context, assertionID string, until time.Time) error {
	if assertionID == "" {
		return errors.New("sso: the assertion carries no ID to record")
	}

	ttl := time.Until(until)
	if ttl <= 0 {
		// Already outside its own validity window. The caller's timing checks
		// will refuse it; recording it would only write a key that expires
		// immediately.
		return nil
	}

	claimed, err := s.client.SetNX(ctx, s.prefix+assertionID, "", ttl).Result()
	if err != nil {
		return err
	}
	if !claimed {
		return ErrAssertionReplayed
	}
	return nil
}

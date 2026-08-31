package sso

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// ErrFlowNotFound reports that a callback presented a state parameter with no
// live flow behind it: it was never issued, it has expired, or -- the case that
// matters -- it has already been used.
var ErrFlowNotFound = errors.New("sso: no login is in progress for this state")

// FlowTTL bounds how long an authorization request may stay outstanding. The
// binding cookie is given the same lifetime, so neither outlives the other. It is
// long enough for somebody to be redirected to their identity provider, type a
// password and answer a second factor, and short enough that a state parameter
// captured from a browser history or a proxy log is worthless by the time
// anybody reads it.
const FlowTTL = 10 * time.Minute

// Flow is the half of an authorization request that never leaves the server.
//
// The nonce and the PKCE verifier are held here rather than in a cookie or in
// the state parameter itself, so a browser cannot be tricked into completing a
// login somebody else started: the only thing the browser carries is an opaque
// state, and the secrets it is checked against are ours.
type Flow struct {
	Nonce        string `json:"nonce"`
	CodeVerifier string `json:"code_verifier"`
	// Binding is the flow's half of the browser binding. Its twin is set on the
	// browser that started the login, as a cookie, and the callback refuses a
	// response the two do not agree on. Without it somebody could authenticate
	// as themselves, stop before following the callback, and hand that URL to
	// another person: the server holds the PKCE verifier and the state is a
	// bearer value, so that browser would be given the attacker's session.
	Binding string `json:"binding"`
	// Redirect is where the console is sent once the session exists. It is
	// validated as a same-site path before it is stored, never on the way out.
	Redirect string `json:"redirect"`
}

// FlowStore holds outstanding authorization requests in Redis, keyed by state.
//
// Redis rather than process memory because a login starts on whichever replica
// answered /start and finishes on whichever answers /callback, and those are not
// the same process.
type FlowStore struct {
	client *goredis.Client
	prefix string
	ttl    time.Duration
}

// NewFlowStore returns nil when there is no Redis client, because a deployment
// that configured no REDIS_ADDRESS has none. Wrapping a nil client in a non-nil
// store would pass the driver's collaborator check and panic at the first
// /start instead of refusing at boot.
func NewFlowStore(client *goredis.Client, prefix string) *FlowStore {
	if client == nil {
		return nil
	}
	return &FlowStore{client: client, prefix: prefix + "auth:oidc:flow:", ttl: FlowTTL}
}

// Save records a flow under its state for the TTL.
func (s *FlowStore) Save(ctx context.Context, state string, flow Flow) error {
	encoded, err := json.Marshal(flow)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.prefix+state, encoded, s.ttl).Err()
}

// Consume reads a flow and deletes it in the same round trip, so a state can be
// redeemed exactly once.
//
// Single use is the whole reason this is GETDEL rather than GET followed by DEL:
// a replayed authorization response -- from a browser back button, a leaked
// referrer, or an attacker who captured the callback URL -- must not be able to
// open a second session.
func (s *FlowStore) Consume(ctx context.Context, state string) (Flow, error) {
	if state == "" {
		return Flow{}, ErrFlowNotFound
	}

	encoded, err := s.client.GetDel(ctx, s.prefix+state).Bytes()
	if errors.Is(err, goredis.Nil) {
		return Flow{}, ErrFlowNotFound
	}
	if err != nil {
		return Flow{}, err
	}

	var flow Flow
	if err := json.Unmarshal(encoded, &flow); err != nil {
		return Flow{}, ErrFlowNotFound
	}
	return flow, nil
}

// NewOpaqueValue returns 32 bytes of CSPRNG output, base64url encoded. It mints
// the state, the nonce and the browser binding; each is only ever compared for
// equality against the copy the server kept.
func NewOpaqueValue() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Package password hashes and verifies the secrets behind local (email +
// password) admin credentials.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// ErrMalformedHash is returned when a stored hash cannot be parsed. It is a
// data-integrity failure, not a wrong password, and callers must not report it
// as one.
var ErrMalformedHash = errors.New("password: malformed hash")

// Params are the argon2id cost parameters a hash was produced with. They are
// recorded in the hash itself, so raising them later leaves every existing hash
// verifiable.
type Params struct {
	// Memory is the size of the memory block in KiB.
	Memory uint32
	// Time is the number of passes over that memory.
	Time uint32
	// Parallelism is the number of lanes.
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultParams is what new hashes are produced with.
//
// argon2id at 64 MiB with two passes over one lane measures at roughly 90ms on
// a current core, which is the target: high enough that offline cracking
// of a leaked hash is expensive, low enough that a login stays interactive and
// that the login endpoint cannot be turned into a memory-exhaustion lever
// (concurrent logins each hold their 64 MiB for the duration of the hash).
//
// The RFC 9106 second recommended configuration is t=3, m=64 MiB, p=4. One lane
// is used instead of four because the work is done on a request goroutine
// alongside everything else the process is serving, where extra lanes buy
// latency at the cost of pinning more cores; the second pass is dropped to keep
// the single-lane cost at the latency target rather than three times it.
var DefaultParams = Params{
	Memory:      64 * 1024,
	Time:        2,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

// Hash derives a PHC-encoded argon2id hash of plain using [DefaultParams].
func Hash(plain string) (string, error) { return HashWith(plain, DefaultParams) }

// HashWith derives a PHC-encoded argon2id hash of plain.
//
// The encoding is self-describing ($argon2id$v=19$m=...,t=...,p=...$salt$hash)
// so the cost parameters can be raised at any point without invalidating hashes
// produced under the old ones; [Verify] reports which hashes are behind.
func HashWith(plain string, params Params) (string, error) {
	salt := make([]byte, params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	key := deriveKey([]byte(plain), salt, params, params.KeyLength)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		params.Memory, params.Time, params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify reports whether plain produces encoded, and whether encoded was made
// with parameters weaker than [DefaultParams] and should be replaced.
//
// outdated is only meaningful when match is true: rehashing on a failed attempt
// would rehash an attacker's guess.
func Verify(encoded, plain string) (match bool, outdated bool, err error) {
	params, salt, want, err := decode(encoded)
	if err != nil {
		return false, false, err
	}

	got := deriveKey([]byte(plain), salt, params, uint32(len(want)))

	// Constant-time: a byte-at-a-time comparison leaks how much of a candidate
	// digest was right, which is enough to reconstruct the digest offline.
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return false, false, nil
	}

	return true, params.weakerThan(DefaultParams), nil
}

// inFlight bounds how many argon2 computations may run at once.
//
// Every one of them allocates Params.Memory — 64 MiB under [DefaultParams] — and
// holds it for the duration. Nothing upstream caps how many arrive together: the
// per-source budget deliberately admits a burst, it counts failures rather than
// attempts so a working password is never charged, and it fails open when Redis
// is unavailable. Unbounded, a few dozen simultaneous logins reserve gigabytes
// and the process is killed for being a memory hog rather than for anything an
// operator can see.
//
// The bound is expressed in concurrent hashes rather than bytes because the cost
// per hash is already fixed by the parameters. It tracks GOMAXPROCS because
// argon2id at p=1 saturates one core per computation, so admitting more than
// there are cores adds latency without adding throughput; the floor keeps a
// single-core container able to sign anybody in, and the ceiling keeps peak
// reservation to 512 MiB on a large host.
var inFlight = make(chan struct{}, maxConcurrent())

func maxConcurrent() int {
	return min(max(runtime.GOMAXPROCS(0), 2), 8)
}

// deriveKey runs the KDF while holding one of the [inFlight] slots.
//
// Waiting for a slot is the correct behaviour rather than refusing: a login that
// queues briefly is a login, and turning a burst into failed sign-ins would hand
// anyone who can generate one a way to lock everybody out. The wait is bounded
// in practice by the work itself — each holder is finished in about 90ms — and
// by whatever deadline the caller's request already carries.
func deriveKey(plain, salt []byte, params Params, length uint32) []byte {
	inFlight <- struct{}{}
	defer func() { <-inFlight }()

	return argon2.IDKey(plain, salt, params.Time, params.Memory, params.Parallelism, length)
}

func (p Params) weakerThan(other Params) bool {
	return p.Memory < other.Memory ||
		p.Time < other.Time ||
		p.Parallelism < other.Parallelism ||
		p.KeyLength < other.KeyLength ||
		p.SaltLength < other.SaltLength
}

func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrMalformedHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return Params{}, nil, nil, ErrMalformedHash
	}
	if version != argon2.Version {
		return Params{}, nil, nil, fmt.Errorf("%w: unsupported argon2 version %d", ErrMalformedHash, version)
	}

	var params Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.Memory, &params.Time, &params.Parallelism); err != nil {
		return Params{}, nil, nil, ErrMalformedHash
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, ErrMalformedHash
	}
	key, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, ErrMalformedHash
	}
	if len(salt) == 0 || len(key) == 0 {
		return Params{}, nil, nil, ErrMalformedHash
	}

	params.SaltLength = uint32(len(salt))
	params.KeyLength = uint32(len(key))

	return params, salt, key, nil
}

// dummyHash is a hash of a value nobody can present. It is built once, lazily,
// so a process that never authenticates a password does not pay for it.
var dummyHash = sync.OnceValue(func() string {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		// A failing CSPRNG is not something this package can carry on through,
		// and it is the same failure Hash would report on the real path.
		panic(err)
	}
	encoded, err := Hash(string(secret))
	if err != nil {
		panic(err)
	}
	return encoded
})

// VerifyDummy spends the work a real verification would, for a credential that
// cannot exist.
//
// It is what an unknown email address is answered with. Without it, "no such
// account" returns in microseconds while a wrong password takes ~100ms, and the
// difference is a free account-enumeration oracle on an endpoint that is
// otherwise careful never to admit which addresses it knows.
func VerifyDummy(plain string) {
	_, _, _ = Verify(dummyHash(), plain)
}

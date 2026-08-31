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

// DefaultParams is what new hashes are produced with. 64 MiB over two passes of
// one lane measures at roughly 90ms on a current core: expensive enough that
// cracking a leaked hash offline is costly, cheap enough that a login stays
// interactive.
var DefaultParams = Params{
	Memory:      64 * 1024,
	Time:        2,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

// Hash derives a PHC-encoded argon2id hash of plain using [DefaultParams].
//
// The encoding is self-describing ($argon2id$v=19$m=...,t=...,p=...$salt$hash),
// so the cost parameters can be raised at any point without invalidating hashes
// produced under the old ones.
func Hash(plain string) (string, error) {
	params := DefaultParams

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

// Verify reports whether plain produces encoded.
func Verify(encoded, plain string) (bool, error) {
	params, salt, want, err := decode(encoded)
	if err != nil {
		return false, err
	}

	got := deriveKey([]byte(plain), salt, params, uint32(len(want)))

	// Constant-time: a byte-at-a-time comparison leaks how much of a candidate
	// digest was right, which is enough to reconstruct the digest offline.
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// inFlight bounds how many argon2 computations run at once. Each reserves
// Params.Memory — 64 MiB under [DefaultParams] — for its duration, and nothing
// upstream caps how many logins arrive together, so an unbounded burst reserves
// gigabytes and the process is killed for being a memory hog.
var inFlight = make(chan struct{}, maxConcurrent())

func maxConcurrent() int {
	return min(max(runtime.GOMAXPROCS(0), 2), 8)
}

// deriveKey runs the KDF holding one of the [inFlight] slots. A burst queues
// rather than failing: turning it into failed sign-ins would hand anyone who
// can generate one a way to lock everybody out.
func deriveKey(plain, salt []byte, params Params, length uint32) []byte {
	inFlight <- struct{}{}
	defer func() { <-inFlight }()

	return argon2.IDKey(plain, salt, params.Time, params.Memory, params.Parallelism, length)
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

	if err := params.usable(); err != nil {
		return Params{}, nil, nil, err
	}

	return params, salt, key, nil
}

// maxDecodedMemory caps the memory a stored hash may ask for, well above
// [DefaultParams] so raising the cost stays possible without a code change, and
// far below anything that would take the process down.
const maxDecodedMemory = 1 << 20 // 1 GiB in KiB

// usable rejects parameters that are syntactically fine and operationally not.
// argon2.IDKey panics on a zero time or parallelism, and a large enough memory
// figure allocates until the process is killed. One malformed row must not turn
// every sign-in on that address into a crash.
func (p Params) usable() error {
	if p.Time == 0 || p.Parallelism == 0 || p.Memory == 0 || p.Memory > maxDecodedMemory {
		return fmt.Errorf("%w: m=%d,t=%d,p=%d", ErrMalformedHash, p.Memory, p.Time, p.Parallelism)
	}
	return nil
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
// difference is a free account-enumeration oracle.
func VerifyDummy(plain string) {
	_, _ = Verify(dummyHash(), plain)
}

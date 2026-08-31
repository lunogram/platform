package password

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHashRoundTrip(t *testing.T) {
	encoded, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}

	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=2,p=1$") {
		t.Errorf("unexpected PHC encoding: %q", encoded)
	}

	match, err := Verify(encoded, "correct horse battery staple")
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if !match {
		t.Error("expected the password to verify")
	}

	match, err = Verify(encoded, "correct horse battery stapl")
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if match {
		t.Error("expected a wrong password to be rejected")
	}
}

// The same password must never produce the same hash twice, or the salt is not
// doing its job and one rainbow table covers every account.
func TestHashIsSalted(t *testing.T) {
	first, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	second, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	if first == second {
		t.Error("two hashes of the same password are identical")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	tests := map[string]string{
		"empty":              "",
		"not phc":            "hunter2",
		"wrong algorithm":    "$argon2i$v=19$m=65536,t=2,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaA",
		"wrong version":      "$argon2id$v=16$m=65536,t=2,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaA",
		"bad parameters":     "$argon2id$v=19$m=abc,t=2,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaA",
		"bad base64 salt":    "$argon2id$v=19$m=65536,t=2,p=1$!!!$aGFzaGhhc2hoYXNoaGFzaA",
		"empty digest":       "$argon2id$v=19$m=65536,t=2,p=1$c2FsdHNhbHRzYWx0c2E$",
		"missing components": "$argon2id$v=19$m=65536,t=2,p=1",
	}

	for name, encoded := range tests {
		t.Run(name, func(t *testing.T) {
			match, err := Verify(encoded, "correct horse battery staple")
			if err == nil {
				t.Fatal("expected an error")
			}
			if !errors.Is(err, ErrMalformedHash) {
				t.Errorf("error = %v, want ErrMalformedHash", err)
			}
			if match {
				t.Error("a malformed hash must never report a match")
			}
		})
	}
}

// VerifyDummy exists purely to burn the same work a real verification would, so
// the only thing to assert is that it runs and does not panic on the lazily
// built hash.
func TestVerifyDummy(t *testing.T) {
	VerifyDummy("anything at all")
	VerifyDummy("anything at all")
}

// Each argon2 computation reserves 64 MiB for its duration, and nothing
// upstream caps how many logins arrive together. The bound is what stops a
// burst from reserving gigabytes.
func TestHashingIsBounded(t *testing.T) {
	assert.GreaterOrEqual(t, cap(inFlight), 2, "a single-core host must still be able to sign anybody in")
	assert.LessOrEqual(t, cap(inFlight), 8, "peak reservation stays bounded on a large host")

	held := cap(inFlight)
	for range held {
		inFlight <- struct{}{}
	}
	defer func() {
		for range held - 1 {
			<-inFlight
		}
	}()

	done := make(chan struct{})
	go func() {
		_, _ = Hash("an entirely ordinary passphrase")
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("hashing ran while every slot was taken")
	case <-time.After(100 * time.Millisecond):
	}

	// Freeing one slot must let it through: a burst queues, it does not fail.
	<-inFlight

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("hashing did not resume once a slot was free")
	}
}

// The parameters come from a stored hash, and argon2.IDKey panics outright on a
// zero time or parallelism while a large enough memory figure allocates until
// the process dies. One malformed row must not turn every sign-in on that
// address into a crash.
func TestVerifyRejectsUnusableParameters(t *testing.T) {
	encoded := map[string]string{
		"zero time":        "$argon2id$v=19$m=65536,t=0,p=1$c29tZXNhbHQ$c29tZWtleXNvbWVrZXlzb21la2V5c29tZWtleQ",
		"zero parallelism": "$argon2id$v=19$m=65536,t=2,p=0$c29tZXNhbHQ$c29tZWtleXNvbWVrZXlzb21la2V5c29tZWtleQ",
		"zero memory":      "$argon2id$v=19$m=0,t=2,p=1$c29tZXNhbHQ$c29tZWtleXNvbWVrZXlzb21la2V5c29tZWtleQ",
		"absurd memory":    "$argon2id$v=19$m=4294967295,t=2,p=1$c29tZXNhbHQ$c29tZWtleXNvbWVrZXlzb21la2V5c29tZWtleQ",
	}

	for name, hash := range encoded {
		t.Run(name, func(t *testing.T) {
			match, err := Verify(hash, "any password at all")
			assert.ErrorIs(t, err, ErrMalformedHash)
			assert.False(t, match)
		})
	}
}

package password

import (
	"errors"
	"strings"
	"testing"
)

func TestHashRoundTrip(t *testing.T) {
	encoded, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}

	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=2,p=1$") {
		t.Errorf("unexpected PHC encoding: %q", encoded)
	}

	match, outdated, err := Verify(encoded, "correct horse battery staple")
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if !match {
		t.Error("expected the password to verify")
	}
	if outdated {
		t.Error("a hash made with the current parameters is not outdated")
	}

	match, _, err = Verify(encoded, "correct horse battery stapl")
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

func TestVerifyReportsOutdatedParameters(t *testing.T) {
	weaker := DefaultParams
	weaker.Memory /= 4
	weaker.Time = 1

	encoded, err := HashWith("correct horse battery staple", weaker)
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}

	match, outdated, err := Verify(encoded, "correct horse battery staple")
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if !match {
		t.Fatal("a hash made with weaker parameters must still verify")
	}
	if !outdated {
		t.Error("expected the weaker hash to be reported as outdated")
	}

	// Rehashing a guess would be worse than useless, so outdated is only ever
	// reported alongside a match.
	_, outdated, err = Verify(encoded, "wrong")
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if outdated {
		t.Error("outdated must not be reported for a failed verification")
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
			match, _, err := Verify(encoded, "correct horse battery staple")
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

func TestValidate(t *testing.T) {
	tests := map[string]struct {
		password string
		email    string
		want     error
	}{
		"long enough":        {password: "an entirely ordinary passphrase", email: "admin@example.com"},
		"exactly minimum":    {password: "abcdefghijkm", email: "admin@example.com"},
		"one short":          {password: "abcdefghijk", email: "admin@example.com", want: ErrTooShort},
		"empty":              {password: "", email: "admin@example.com", want: ErrTooShort},
		"too long":           {password: strings.Repeat("a", MaxLength+1), email: "admin@example.com", want: ErrTooLong},
		"is the address":     {password: "admin@example.com", email: "admin@example.com", want: ErrSimilar},
		"contains the local": {password: "sysadministrator99", email: "sysadministrator@example.com", want: ErrSimilar},
		"differs in case":    {password: "ADMIN@EXAMPLE.COM", email: "admin@example.com", want: ErrSimilar},
		"is the domain":      {password: "exampleexample", email: "admin@exampleexample.com", want: ErrSimilar},
		"common":             {password: "passwordpassword", email: "admin@example.com", want: ErrCommon},
		"common other case":  {password: "PasswordPassword", email: "admin@example.com", want: ErrCommon},
		// A short local part must not reject every password containing it, or
		// "bo@example.com" cannot use a password containing "bo".
		"short local part": {password: "a thoroughly boring passphrase", email: "bo@example.com"},
		// Multi-byte characters count as one rune each; counting bytes would let
		// a four-character password through.
		"unicode counted by rune": {password: "パスワード", email: "admin@example.com", want: ErrTooShort},
		"unicode long enough":     {password: "パスワードパスワードパスワード", email: "admin@example.com"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := Validate(test.password, test.email)
			if !errors.Is(err, test.want) {
				t.Errorf("Validate() = %v, want %v", err, test.want)
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

package password

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// MinLength is the shortest password accepted.
//
// Twelve is chosen over the more common eight because length is the only knob
// that actually buys entropy here: composition rules (one upper, one digit, one
// symbol) are deliberately absent, since in practice they push people towards a
// small set of predictable substitutions and measurably REDUCE the entropy of
// the passwords they produce.
const MinLength = 12

// MaxLength caps the input before it reaches argon2. The hash cost does not
// grow with the input, but the request body and the memory the candidate is
// copied through do, and no legitimate password approaches this.
const MaxLength = 1024

var (
	ErrTooShort = errors.New("password: shorter than the minimum length")
	ErrTooLong  = errors.New("password: longer than the maximum length")
	ErrSimilar  = errors.New("password: too similar to the email address")
	ErrCommon   = errors.New("password: too easily guessed")
)

// Validate applies the password policy.
//
// The rules are length, "not the address you are signing in with", and a small
// set of long strings that appear in every breach corpus.
//
// There is deliberately no large breached-password list. Almost every entry in
// a top-N corpus is shorter than [MinLength] and therefore already unreachable,
// so embedding one would add tens of thousands of lines that reject nothing
// this does not, in exchange for a dependency or a generated blob nobody
// maintains. The entries below are the long ones that survive the length rule.
func Validate(plain, email string) error {
	length := utf8.RuneCountInString(plain)
	if length < MinLength {
		return ErrTooShort
	}
	if length > MaxLength {
		return ErrTooLong
	}

	if similarToEmail(plain, email) {
		return ErrSimilar
	}

	if isCommon(plain) {
		return ErrCommon
	}

	return nil
}

// similarToEmail rejects a password built out of the address it protects. Such
// a password is public knowledge the moment the address is: it is the first
// thing a targeted guess tries, and no amount of length helps.
func similarToEmail(plain, email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false
	}
	lowered := strings.ToLower(plain)

	if strings.Contains(lowered, email) || strings.Contains(email, lowered) {
		return true
	}

	local, domain, _ := strings.Cut(email, "@")
	if len(local) >= 4 && strings.Contains(lowered, local) {
		return true
	}
	// The registrable part of the domain only; "com" or "co" would reject far
	// too much on their own.
	if host, _, _ := strings.Cut(domain, "."); len(host) >= 4 && lowered == host {
		return true
	}

	return false
}

// commonPasswords are the long strings that recur across public breach corpora:
// keyboard walks, repeated words, and the phrases people reach for when a form
// tells them twelve characters. Compared case-insensitively.
var commonPasswords = map[string]struct{}{
	"123456789012":     {},
	"1234567890123":    {},
	"12345678901234":   {},
	"123456789012345":  {},
	"1234567890abc":    {},
	"qwertyuiopasdfgh": {},
	"qwertyuiop123":    {},
	"qwertyuiop[]":     {},
	"1qaz2wsx3edc":     {},
	"1q2w3e4r5t6y":     {},
	"zaq12wsxcde3":     {},
	"passwordpassword": {},
	"password123456":   {},
	"password1234":     {},
	"passw0rdpassw0rd": {},
	"letmeinletmein":   {},
	"iloveyou1234":     {},
	"iloveyouiloveyou": {},
	"trustno1trustno1": {},
	"administrator":    {},
	"administrator1":   {},
	"changeme1234":     {},
	"changemechangeme": {},
	"welcome123456":    {},
	"welcomewelcome":   {},
	"secretsecret":     {},
	"whatthefuck12":    {},
	"aaaaaaaaaaaa":     {},
	"abcdefghijkl":     {},
	"abcdefghijklm":    {},
	"abcd1234abcd":     {},
	"asdfghjklzxcv":    {},
	"monkeymonkey":     {},
	"superman1234":     {},
	"football1234":     {},
	"princess1234":     {},
	"sunshine1234":     {},
	"lunogramlunogram": {},
	"lunogram1234":     {},
}

func isCommon(plain string) bool {
	_, found := commonPasswords[strings.ToLower(plain)]
	return found
}

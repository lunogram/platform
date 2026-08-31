// Package configfile holds the two things every configuration source in the
// platform needs: interpolation of ${NAME} references from the environment, and
// resolution of a base64://, file:// or literal reference to its bytes.
//
// It is a leaf package on purpose. The node configuration, the outbound hook
// engine and the mailer all consume operator-authored files, and a shared
// mechanism is what keeps "how do I point this at a file" from having a
// different answer in each of them.
package configfile

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const (
	// FileScheme marks a reference that points at a file on disk.
	FileScheme = "file://"
	// Base64Scheme marks a reference whose payload is base64-encoded inline.
	//
	// This is the only form that survives ${NAME} interpolation intact.
	// Interpolation is textual and happens before the YAML is parsed, so a value
	// carrying newlines, quotes or a colon would not land in a scalar — it would
	// reshape the document. The base64 alphabet contains no character YAML
	// treats specially, which makes an environment variable a safe carrier for
	// something as large and as punctuated as an HTML email.
	Base64Scheme = "base64://"
)

// envRef matches a ${NAME} interpolation.
var envRef = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Interpolate expands ${NAME} references from the process environment.
//
// Secrets belong in the environment, not in a file that ships in a ConfigMap or
// a git repository, so the schema carries credential *references* and this
// resolves them. An unset variable is an error rather than an empty string:
// silently sending an empty API key produces a 401 at 3am instead of a failure
// at boot.
//
// A value carrying a line break is rejected. Interpolation happens on the raw
// bytes before they are parsed, so a newline does not land inside a scalar — it
// ends the line and makes the rest of the value into whatever the next line
// happens to parse as. There is no value for which that is the intent.
//
// A '#' is not rejected, because it is legitimate in a password, but YAML reads
// one that follows whitespace as the start of a comment. Quote the placeholder
// in the file ("${NAME}") and the hazard goes away, which is what the shipped
// examples do.
func Interpolate(raw []byte) ([]byte, error) {
	var missing []string
	var multiline []string

	out := envRef.ReplaceAllFunc(raw, func(match []byte) []byte {
		name := string(envRef.FindSubmatch(match)[1])
		value, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, name)
			return nil
		}
		if strings.ContainsAny(value, "\r\n") {
			multiline = append(multiline, name)
			return nil
		}
		return []byte(value)
	})

	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("unset environment variables referenced by config: %s", strings.Join(missing, ", "))
	}
	if len(multiline) > 0 {
		sort.Strings(multiline)
		return nil, fmt.Errorf(
			"environment variables referenced by config carry a line break, which would corrupt the document: %s (base64-encode the value and reference it as %s${NAME})",
			strings.Join(multiline, ", "), Base64Scheme)
	}

	return out, nil
}

// Resolve turns a configuration reference into its bytes.
//
//	base64://<payload>   decoded inline
//	file://<path>        read from disk, relative paths against baseDir
//	anything else        used literally
//
// name identifies the setting in any error, because "invalid base64" without
// saying which of a dozen templates it came from is not something an operator
// can act on.
func Resolve(name, ref, baseDir string) ([]byte, error) {
	if payload, ok := strings.CutPrefix(ref, Base64Scheme); ok {
		// GNU base64 wraps at 76 columns unless it is told not to, so an
		// operator who pipes a file through it and pastes the result hands us a
		// payload with line breaks in it. Accepting that is cheaper than
		// teaching everyone the -w0 flag.
		payload = strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, payload)

		if payload == "" {
			return nil, fmt.Errorf("%s: %s reference has no payload", name, Base64Scheme)
		}

		decoded, err := decodeBase64(payload)
		if err != nil {
			return nil, fmt.Errorf("%s: %s payload is not valid base64: %w", name, Base64Scheme, err)
		}
		return decoded, nil
	}

	if path, ok := strings.CutPrefix(ref, FileScheme); ok {
		if path == "" {
			return nil, fmt.Errorf("%s: %s reference has no path", name, FileScheme)
		}
		if !filepath.IsAbs(path) && baseDir != "" {
			path = filepath.Join(baseDir, path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return raw, nil
	}

	return []byte(ref), nil
}

// decodeBase64 accepts both the padded and the unpadded form. Operators produce
// these with different tools and the difference is not one worth a support
// thread.
func decodeBase64(payload string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(payload); err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(payload)
}

// Package configfile holds the two things every configuration source in the
// platform needs: expansion of ${NAME} references from the environment, and
// resolution of a base64://, file:// or literal reference to its bytes.
//
// It is a leaf package on purpose. The node configuration, the outbound hook
// engine and the mailer all consume operator-authored files, and a shared
// mechanism is what keeps "how do I point this at a file" from having a
// different answer in each of them.
package configfile

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const (
	// FileScheme marks a reference that points at a file on disk.
	FileScheme = "file://"
	// Base64Scheme marks a reference whose payload is base64-encoded inline.
	//
	// It earns its place on large values. An HTML email is a few hundred lines
	// of markup: inline it has to become a YAML block scalar and stay correctly
	// indented, and through an environment variable it has to survive whatever
	// the surrounding shell, ConfigMap or secret store does to line breaks.
	// Base64 makes it one opaque token that none of those can reshape, and
	// file:// is the better answer once it is big enough to want editing.
	Base64Scheme = "base64://"
)

// envRef matches a ${NAME} reference, and the $${NAME} form that escapes one.
var envRef = regexp.MustCompile(`\$?\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Decode parses raw as YAML, expands the ${NAME} references in its scalar
// values from the environment, and decodes the result into out.
//
// Expansion happens on the parsed document rather than on the bytes, and that
// is the whole design. Substituting into the text before it is parsed means a
// value decides how the rest of the file is read: a line break ends the line
// and whatever follows is parsed as though the operator had written it, a '#'
// starts a comment that swallows the rest of the value, and a ':' invents a
// mapping. Substituting into a scalar node that has already been parsed cannot
// do any of that -- the value is data by the time it exists, so a secret is
// free to contain whatever a secret contains.
//
// Decoding rejects keys the target does not have. An unrecognised key is a
// typo, and a typo in configuration is a setting that silently did not apply.
func Decode(raw []byte, out any) error {
	var document yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return err
	}

	// An empty document leaves out untouched rather than zeroing it, so an
	// empty file is "configure nothing" rather than "unset everything".
	if document.Kind == 0 {
		return nil
	}

	if err := expand(&document); err != nil {
		return err
	}

	// Re-encoding is what turns an expanded value back into well-formed YAML:
	// the encoder quotes whatever needs quoting, so a value that looks like an
	// alias, an anchor, a merge key or a nested mapping is emitted as the
	// string it is.
	expanded, err := yaml.Marshal(&document)
	if err != nil {
		return err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(expanded))
	decoder.KnownFields(true)
	if err := decoder.Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// expand walks the document and substitutes into scalar values.
func expand(node *yaml.Node) error {
	var missing []string
	var keys []string

	var walk func(*yaml.Node, bool)
	walk = func(n *yaml.Node, isKey bool) {
		if n == nil {
			return
		}

		switch n.Kind {
		case yaml.ScalarNode:
			if !envRef.MatchString(n.Value) {
				return
			}
			if isKey {
				// A key assembled from the environment could rename or shadow a
				// setting, which is a different and much sharper thing than
				// supplying a value. Keys are the operator's alone.
				keys = append(keys, n.Value)
				return
			}
			substitute(n, &missing)

		case yaml.MappingNode:
			// Content alternates key, value. Inside a key, everything stays a
			// key: a mapping used as one contributes its values to the key's
			// identity.
			for i, child := range n.Content {
				walk(child, isKey || i%2 == 0)
			}

		case yaml.AliasNode:
			// The anchor this points at is walked where it is defined. Following
			// the alias as well would expand it twice.

		default:
			// isKey is carried down rather than reset, so a complex key -- a
			// sequence or mapping used as one -- is still a key all the way
			// through.
			for _, child := range n.Content {
				walk(child, isKey)
			}
		}
	}
	walk(node, false)

	if len(keys) > 0 {
		sort.Strings(keys)
		return fmt.Errorf("configuration keys cannot be built from the environment: %s", strings.Join(keys, ", "))
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("unset environment variables referenced by config: %s", strings.Join(missing, ", "))
	}
	return nil
}

// substitute replaces every reference in one scalar.
func substitute(node *yaml.Node, missing *[]string) {
	original := node.Value

	node.Value = envRef.ReplaceAllStringFunc(original, func(match string) string {
		// $${NAME} is an escaped reference: it yields the literal text, which is
		// what an inline JSONNet body or a shell snippet in the file needs.
		if strings.HasPrefix(match, "$$") {
			return match[1:]
		}

		name := envRef.FindStringSubmatch(match)[1]
		value, ok := os.LookupEnv(name)
		if !ok {
			*missing = append(*missing, name)
			return ""
		}
		// The replacement is not rescanned: a variable holding ${OTHER} yields
		// that text rather than reaching for another variable.
		return value
	})

	// A plain scalar was written without quotes, so its type comes from what it
	// says -- `port: ${PORT}` has to end up an integer. Re-resolving the tag
	// against the new value is what makes that work, and it is confined to plain
	// scalars: an operator who wrote quotes asked for a string and gets one.
	if node.Value != original && node.Style == 0 {
		node.Tag = ""
	}
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

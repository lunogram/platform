package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/caarlos0/env/v10"
	"github.com/lunogram/platform/internal/configfile"
	"gopkg.in/yaml.v3"
)

// ConfigFileEnv names the environment variable holding the path to the node
// configuration file. It is the one setting that cannot itself live in the
// file.
const ConfigFileEnv = "CONFIG_FILE"

// Load builds the node configuration from its three layers, in order:
//
//  1. Defaults()      — what a deployment gets before it configures anything
//  2. the YAML file   — CONFIG_FILE, if one is set
//  3. the environment — every env-tagged field whose variable is set
//
// Later layers win, so a single environment variable can override one setting
// out of a file without restating the rest of it. That ordering is why the
// defaults are a constructor rather than envDefault tags: a tag default is
// applied whenever its variable is unset, which would put layer 1 on top of
// layer 2.
//
// The two layers are not equally expressive. The file is the complete surface;
// the environment can only reach scalar settings, because a hook set — many
// subscribers per event, each with its own template, credential and retry
// policy — has no sensible flat encoding.
func Load() (Node, error) {
	cfg := Defaults()

	if path := os.Getenv(ConfigFileEnv); path != "" {
		if err := cfg.applyFile(path); err != nil {
			return Node{}, err
		}
	}

	if err := env.Parse(&cfg); err != nil {
		return Node{}, err
	}

	return cfg, nil
}

// applyFile overlays a YAML document onto the receiver. Keys absent from the
// document leave the layer beneath them untouched, which is what makes the file
// an overlay rather than a replacement.
func (n *Node) applyFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	expanded, err := configfile.Interpolate(raw)
	if err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(expanded))
	// An unrecognised key is a typo, and a typo in configuration is a setting
	// that silently did not apply. Rejecting it at boot is the whole reason the
	// file is read before anything is served.
	dec.KnownFields(true)
	if err := dec.Decode(n); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}

	n.baseDir = filepath.Dir(path)

	// A hook configuration declared inline was never a file of its own, so it
	// has no directory to resolve relative file:// references against until it
	// is told the one this document came from.
	n.Webhook.Outbound.SetBaseDir(n.baseDir)

	return nil
}

// BaseDir is the directory the configuration file was read from. Relative
// file:// references resolve against it, so a configuration and the templates
// it points at can be shipped together and moved together.
func (n Node) BaseDir() string { return n.baseDir }

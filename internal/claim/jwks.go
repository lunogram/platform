package claim

import (
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"gopkg.in/yaml.v3"
)

type JWKS struct {
	keyfunc jwt.Keyfunc
}

func (jwks *JWKS) UnmarshalText(server []byte) error {
	store, err := keyfunc.NewDefault([]string{string(server)})
	if err != nil {
		return err
	}

	jwks.keyfunc = store.Keyfunc
	return nil
}

func (jwks JWKS) Unwrap() jwt.Keyfunc {
	return jwks.keyfunc
}

// UnmarshalYAML lets a JWKS be configured from the YAML file as well as from
// the environment. The environment path goes through [JWKS.UnmarshalText],
// which yaml.v3 does not consult on its own — it looks only for its own
// interface — so this delegates rather than duplicating the fetch.
func (jwks *JWKS) UnmarshalYAML(value *yaml.Node) error {
	var server string
	if err := value.Decode(&server); err != nil {
		return err
	}
	if server == "" {
		return nil
	}
	return jwks.UnmarshalText([]byte(server))
}

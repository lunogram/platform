package claim

import (
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
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

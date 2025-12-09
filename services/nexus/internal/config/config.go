package config

import (
	"github.com/lunogram/platform/pkg/claim"
	"github.com/lunogram/platform/pkg/http"
	"github.com/lunogram/platform/services/nexus/internal/storage"
	"github.com/lunogram/platform/services/nexus/internal/store"
)

type Service struct {
	JWTSecret                string     `env:"JWT_SECRET"`
	JWKS                     claim.JWKS `env:"JWKS_URL"`
	ManagementServiceAddress string     `env:"ADMIN_SERVICE_ADDRESS" envDefault:":8080"`
	PublicServiceAddress     string     `env:"PUBLIC_SERVICE_ADDRESS" envDefault:":8081"`
	PlatformURL              string     `env:"PLATFORM_URL" envDefault:"http://localhost:3001"`
	HTTP                     http.Config
	Store                    store.Config
	Storage                  storage.Config
}

package container

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/azure/azurite"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Azurite serves a single well-known development account, whose name and key
// are fixed and published by Microsoft.
const (
	AzuriteAccount    = azurite.AccountName
	AzuriteAccountKey = azurite.AccountKey
)

var azuriteOnce struct {
	sync.Once
	endpoint string
	err      error
}

// RunAzurite runs an Azurite container for testing and returns the blob
// service endpoint. Azurite is Microsoft's Azure Storage emulator. The
// endpoint carries the account path segment, as the blob service URL does on
// Azure Stack and other non-public deployments.
//
// The container is booted once per test binary rather than reused by name
// across runs: Azurite is reaped between runs regardless, so a named container
// buys nothing and instead races the previous run's teardown.
func RunAzurite(t *testing.T) (endpoint string) {
	t.Helper()

	azuriteOnce.Do(func() {
		ctx := context.Background()

		container, err := azurite.Run(ctx,
			"mcr.microsoft.com/azure-storage/azurite:3.37.0",
			azurite.WithEnabledServices(azurite.BlobService),
			testcontainers.WithAdditionalWaitStrategy(blobServiceServing()),
		)
		if err != nil {
			azuriteOnce.err = err
			return
		}

		blobServiceURL, err := container.BlobServiceURL(ctx)
		if err != nil {
			azuriteOnce.err = err
			return
		}

		azuriteOnce.endpoint = blobServiceURL + "/" + AzuriteAccount
	})

	require.NoError(t, azuriteOnce.err)
	return azuriteOnce.endpoint
}

// blobServiceServing waits until Azurite answers an HTTP request, rather than
// until its port merely accepts a connection. Docker's port proxy accepts on
// the published port as soon as the container is created, so the module's own
// wait.ForListeningPort is satisfied while the Node process inside is still
// starting — the first request then reads a closed connection ("EOF", or
// "server closed idle connection") instead of a response.
//
// Any 4xx answers the only question being asked: the blob service parsed a
// request and rejected it. The probe is deliberately unauthenticated and
// account-less, so it cannot depend on the account being provisioned yet;
// Azurite replies 400 to "GET /".
func blobServiceServing() wait.Strategy {
	return wait.ForHTTP("/").
		WithPort(azurite.BlobPort).
		WithMethod(http.MethodGet).
		WithStatusCodeMatcher(func(status int) bool {
			return status >= 400 && status < 500
		}).
		WithPollInterval(200 * time.Millisecond).
		WithStartupTimeout(2 * time.Minute)
}

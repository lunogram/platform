package container

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/azure/azurite"
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

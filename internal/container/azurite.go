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
	sync.Mutex
	endpoint string
}

const azuriteStartAttempts = 3

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

	azuriteOnce.Lock()
	defer azuriteOnce.Unlock()

	if azuriteOnce.endpoint != "" {
		return azuriteOnce.endpoint
	}

	var err error
	for attempt := 1; attempt <= azuriteStartAttempts; attempt++ {
		endpoint, err = startAzurite(context.Background())
		if err == nil {
			azuriteOnce.endpoint = endpoint
			return azuriteOnce.endpoint
		}

		t.Logf("azurite startup attempt %d/%d failed: %v", attempt, azuriteStartAttempts, err)
		time.Sleep(time.Duration(attempt) * time.Second)
	}

	require.NoError(t, err)
	return azuriteOnce.endpoint
}

func startAzurite(ctx context.Context) (string, error) {
	container, err := azurite.Run(ctx,
		"mcr.microsoft.com/azure-storage/azurite:3.37.0",
		azurite.WithEnabledServices(azurite.BlobService),
		testcontainers.WithAdditionalWaitStrategy(blobServiceServing()),
	)
	if err != nil {
		// A container that started but failed its wait strategy is still
		// returned; remove it so retries do not pile up running containers.
		_ = testcontainers.TerminateContainer(container)
		return "", err
	}

	blobServiceURL, err := container.BlobServiceURL(ctx)
	if err != nil {
		_ = testcontainers.TerminateContainer(container)
		return "", err
	}

	return blobServiceURL + "/" + AzuriteAccount, nil
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

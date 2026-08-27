package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
)

type AzureConfig struct {
	Account    string `env:"ACCOUNT"`
	Container  string `env:"CONTAINER"`
	AccountKey string `env:"ACCOUNT_KEY"`
	// Endpoint overrides the blob service URL. Leave it empty for the public
	// Azure cloud; set it for sovereign clouds, Azure Stack, a private
	// endpoint, or a local Azurite emulator, in which case it must include
	// the account path segment (e.g. http://127.0.0.1:10000/devstoreaccount1).
	Endpoint string `env:"ENDPOINT"`
}

// ServiceURL returns the blob service URL to address the account on,
// defaulting to the public Azure cloud when no endpoint override is set.
func (cnf AzureConfig) ServiceURL() string {
	if cnf.Endpoint != "" {
		return strings.TrimRight(cnf.Endpoint, "/")
	}

	return fmt.Sprintf("https://%s.blob.core.windows.net", cnf.Account)
}

func NewAzureStorage(cnf AzureConfig) (*AzureStorage, error) {
	if cnf.Account == "" {
		return nil, fmt.Errorf("azure storage account is required")
	}

	if cnf.Container == "" {
		return nil, fmt.Errorf("azure storage container is required")
	}

	if cnf.AccountKey == "" {
		return nil, fmt.Errorf("azure storage account key is required")
	}

	credential, err := azblob.NewSharedKeyCredential(cnf.Account, cnf.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to build Azure credentials: %w", err)
	}

	client, err := azblob.NewClientWithSharedKeyCredential(cnf.ServiceURL(), credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}

	return &AzureStorage{
		client:    client,
		container: cnf.Container,
	}, nil
}

type AzureStorage struct {
	client    *azblob.Client
	container string
}

func (s *AzureStorage) Write(ctx context.Context, key string, reader io.Reader, contentType string) error {
	options := &azblob.UploadStreamOptions{}
	if contentType != "" {
		options.HTTPHeaders = &blob.HTTPHeaders{BlobContentType: &contentType}
	}

	_, err := s.client.UploadStream(ctx, s.container, key, reader, options)
	if err != nil {
		return fmt.Errorf("failed to upload to Azure Blob Storage: %w", err)
	}

	return nil
}

func (s *AzureStorage) Read(ctx context.Context, key string) (io.ReadCloser, error) {
	response, err := s.client.DownloadStream(ctx, s.container, key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob from Azure Blob Storage: %w", err)
	}

	return response.Body, nil
}

func (s *AzureStorage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteBlob(ctx, s.container, key, nil)
	if bloberror.HasCode(err, bloberror.BlobNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to delete blob from Azure Blob Storage: %w", err)
	}

	return nil
}

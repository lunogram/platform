package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Config struct {
	Bucket    string `env:"BUCKET"`
	Region    string `env:"REGION"`
	Endpoint  string `env:"ENDPOINT"`
	AccessKey string `env:"ACCESS_KEY"`
	SecretKey string `env:"SECRET_KEY"`
}

func NewS3Storage(bucket S3Config) (*S3Storage, error) {
	var opts []func(*config.LoadOptions) error

	if bucket.Region != "" {
		opts = append(opts, config.WithRegion(bucket.Region))
	}

	if bucket.AccessKey != "" && bucket.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(bucket.AccessKey, bucket.SecretKey, ""),
		))
	}

	config, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	var options []func(*s3.Options)
	if bucket.Endpoint != "" {
		options = append(options, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(bucket.Endpoint)
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(config, options...)

	return &S3Storage{
		client: client,
		bucket: bucket.Bucket,
	}, nil
}

type S3Storage struct {
	client *s3.Client
	bucket string
}

func (s *S3Storage) Write(ctx context.Context, key string, reader io.Reader, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   reader,
	}

	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

func (s *S3Storage) Read(ctx context.Context, key string) (io.ReadCloser, error) {
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	output, err := s.client.GetObject(ctx, getInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}

	return output.Body, nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	deleteInput := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObject(ctx, deleteInput)
	if err != nil {
		return fmt.Errorf("failed to delete object from S3: %w", err)
	}

	return nil
}

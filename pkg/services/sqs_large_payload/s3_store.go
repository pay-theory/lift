package sqslargepayload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	ErrNilS3Client = errors.New("sqs large payload s3 store client is nil")
	ErrInvalidRef  = errors.New("sqs large payload ref missing bucket or key")
)

// S3API defines the subset of the AWS SDK S3 client used by Lift's large payload helpers.
type S3API interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// S3Store implements ObjectStore using AWS S3 (AWS SDK v2).
type S3Store struct {
	Client S3API
}

// Put stores the payload at the specified bucket + key.
func (s S3Store) Put(ctx context.Context, ref Ref, payload []byte) error {
	if s.Client == nil {
		return ErrNilS3Client
	}
	if err := validateRef(ref); err != nil {
		return err
	}

	_, err := s.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &ref.Bucket,
		Key:    &ref.Key,
		Body:   bytes.NewReader(payload),
	})
	return err
}

// Get retrieves the payload bytes from the specified bucket + key.
func (s S3Store) Get(ctx context.Context, ref Ref) (payload []byte, err error) {
	if s.Client == nil {
		return nil, ErrNilS3Client
	}
	if validateErr := validateRef(ref); validateErr != nil {
		return nil, validateErr
	}

	out, err := s.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &ref.Bucket,
		Key:    &ref.Key,
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := out.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	payload, err = io.ReadAll(out.Body)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

// Delete deletes the payload object from the specified bucket + key.
func (s S3Store) Delete(ctx context.Context, ref Ref) error {
	if s.Client == nil {
		return ErrNilS3Client
	}
	if err := validateRef(ref); err != nil {
		return err
	}

	_, err := s.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &ref.Bucket,
		Key:    &ref.Key,
	})
	return err
}

func validateRef(ref Ref) error {
	if ref.Bucket == "" || ref.Key == "" {
		return fmt.Errorf("%w: bucket=%q key=%q", ErrInvalidRef, ref.Bucket, ref.Key)
	}
	return nil
}

package sqslargepayload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestS3Store_NilClient(t *testing.T) {
	store := S3Store{}
	ref := Ref{Bucket: "b", Key: "k"}

	if err := store.Put(context.Background(), ref, []byte("x")); !errors.Is(err, ErrNilS3Client) {
		t.Fatalf("expected ErrNilS3Client, got %v", err)
	}
	if _, err := store.Get(context.Background(), ref); !errors.Is(err, ErrNilS3Client) {
		t.Fatalf("expected ErrNilS3Client, got %v", err)
	}
	if err := store.Delete(context.Background(), ref); !errors.Is(err, ErrNilS3Client) {
		t.Fatalf("expected ErrNilS3Client, got %v", err)
	}
}

func TestS3Store_InvalidRef(t *testing.T) {
	client := newFakeS3()
	store := S3Store{Client: client}

	if err := store.Put(context.Background(), Ref{}, []byte("x")); !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("expected ErrInvalidRef, got %v", err)
	}
	if _, err := store.Get(context.Background(), Ref{}); !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("expected ErrInvalidRef, got %v", err)
	}
	if err := store.Delete(context.Background(), Ref{}); !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("expected ErrInvalidRef, got %v", err)
	}
}

func TestS3Store_PutGetDelete(t *testing.T) {
	ctx := context.Background()
	client := newFakeS3()
	store := S3Store{Client: client}
	ref := Ref{Bucket: "bucket", Key: "key"}
	payload := []byte("payload")

	if err := store.Put(ctx, ref, payload); err != nil {
		t.Fatalf("unexpected put error: %v", err)
	}

	got, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("expected payload %q got %q", payload, got)
	}

	if err := store.Delete(ctx, ref); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}
	if _, err := store.Get(ctx, ref); err == nil {
		t.Fatalf("expected get to fail after delete")
	}
}

type fakeS3 struct {
	objects map[Ref][]byte
}

func newFakeS3() *fakeS3 {
	return &fakeS3{objects: make(map[Ref][]byte)}
}

func (f *fakeS3) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if params == nil || params.Bucket == nil || params.Key == nil || params.Body == nil {
		return nil, errors.New("invalid input")
	}
	body, err := io.ReadAll(params.Body)
	if err != nil {
		return nil, err
	}
	f.objects[Ref{Bucket: *params.Bucket, Key: *params.Key}] = append([]byte(nil), body...)
	return &s3.PutObjectOutput{}, nil
}

func (f *fakeS3) GetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if params == nil || params.Bucket == nil || params.Key == nil {
		return nil, errors.New("invalid input")
	}
	payload, ok := f.objects[Ref{Bucket: *params.Bucket, Key: *params.Key}]
	if !ok {
		return nil, errors.New("not found")
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(payload)),
	}, nil
}

func (f *fakeS3) DeleteObject(_ context.Context, params *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if params == nil || params.Bucket == nil || params.Key == nil {
		return nil, errors.New("invalid input")
	}
	delete(f.objects, Ref{Bucket: *params.Bucket, Key: *params.Key})
	return &s3.DeleteObjectOutput{}, nil
}

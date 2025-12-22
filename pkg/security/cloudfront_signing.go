package security

import (
	"bytes"
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/cloudfront/sign"
)

// CloudFrontSigner creates signed URLs and cookies for CloudFront private content.
//
// It is a thin wrapper around aws-sdk-go-v2's CloudFront signing helpers, wired to
// Lift's SecretsProvider for loading private keys at runtime.
type CloudFrontSigner struct {
	keyPairID string
	privKey   *rsa.PrivateKey
	urlSigner *sign.URLSigner
}

// CloudFrontCookieOptions configures optional attributes on signed cookies.
type CloudFrontCookieOptions struct {
	Path     string
	Domain   string
	Secure   bool
	SameSite http.SameSite

	// Optional: if set, applies Expires to all cookies.
	Expires time.Time
}

// NewCloudFrontSigner creates a signer from an RSA private key PEM (PKCS#1 or PKCS#8).
func NewCloudFrontSigner(keyPairID string, privateKeyPEM []byte) (*CloudFrontSigner, error) {
	if keyPairID == "" {
		return nil, fmt.Errorf("cloudfront key pair id is required")
	}
	if len(privateKeyPEM) == 0 {
		return nil, fmt.Errorf("cloudfront private key PEM is required")
	}

	priv, err := sign.LoadPEMPrivKey(bytes.NewReader(privateKeyPEM))
	if err != nil {
		parsed, pkcs8Err := sign.LoadPEMPrivKeyPKCS8(bytes.NewReader(privateKeyPEM))
		if pkcs8Err != nil {
			return nil, fmt.Errorf("parse cloudfront private key (pkcs1: %v, pkcs8: %v)", err, pkcs8Err)
		}
		rsaKey, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("cloudfront private key must be RSA (got %T)", parsed)
		}
		priv = rsaKey
	}

	return &CloudFrontSigner{
		keyPairID: keyPairID,
		privKey:   priv,
		urlSigner: sign.NewURLSigner(keyPairID, priv),
	}, nil
}

// NewCloudFrontSignerFromSecrets loads the private key PEM from Lift's SecretsProvider and returns a signer.
func NewCloudFrontSignerFromSecrets(ctx context.Context, secrets SecretsProvider, keyPairID, secretName string) (*CloudFrontSigner, error) {
	if secrets == nil {
		return nil, fmt.Errorf("secrets provider is required")
	}
	if secretName == "" {
		return nil, fmt.Errorf("cloudfront private key secret name is required")
	}

	pem, err := secrets.GetSecret(ctx, secretName)
	if err != nil {
		return nil, fmt.Errorf("get cloudfront private key secret: %w", err)
	}

	return NewCloudFrontSigner(keyPairID, []byte(pem))
}

// SignURL returns a signed URL using CloudFront's canned policy (expiry only).
func (s *CloudFrontSigner) SignURL(rawURL string, expires time.Time) (string, error) {
	if s == nil || s.urlSigner == nil {
		return "", fmt.Errorf("cloudfront signer is nil")
	}
	return s.urlSigner.Sign(rawURL, expires)
}

// SignCookies returns signed cookies for the provided resource wildcard (e.g., "https://media.example.com/private/*").
func (s *CloudFrontSigner) SignCookies(resource string, expires time.Time, opts *CloudFrontCookieOptions) ([]*http.Cookie, error) {
	if s == nil || s.privKey == nil {
		return nil, fmt.Errorf("cloudfront signer is nil")
	}

	cookieSigner := sign.NewCookieSigner(s.keyPairID, s.privKey)
	var optionFns []func(*sign.CookieOptions)
	if opts != nil {
		optionFns = append(optionFns, func(o *sign.CookieOptions) {
			o.Path = opts.Path
			o.Domain = opts.Domain
			o.Secure = opts.Secure
			o.SameSite = opts.SameSite
			o.Expires = opts.Expires
		})
	}

	return cookieSigner.Sign(resource, expires, optionFns...)
}


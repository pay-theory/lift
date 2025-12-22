package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCloudFrontSigner_SignsURLAndCookies(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})

	signer, err := NewCloudFrontSigner("K123EXAMPLE", privPEM)
	require.NoError(t, err)

	signedURL, err := signer.SignURL("https://media.example.com/private/video.mp4", time.Unix(1700000000, 0))
	require.NoError(t, err)
	require.Contains(t, signedURL, "Expires=")
	require.Contains(t, signedURL, "Signature=")
	require.Contains(t, signedURL, "Key-Pair-Id=K123EXAMPLE")

	cookies, err := signer.SignCookies("http*://media.example.com/private/*", time.Unix(1700000000, 0), &CloudFrontCookieOptions{
		Domain: ".example.com",
		Path:   "/",
		Secure: true,
	})
	require.NoError(t, err)
	require.Len(t, cookies, 3)
	require.Equal(t, "CloudFront-Policy", cookies[0].Name)
	require.Equal(t, "CloudFront-Signature", cookies[1].Name)
	require.Equal(t, "CloudFront-Key-Pair-Id", cookies[2].Name)
}


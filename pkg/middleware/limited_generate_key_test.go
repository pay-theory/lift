package middleware

import (
	"context"
	"testing"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestGenerateKey_PrefersUserTenantThenIP(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"X-Forwarded-For": "10.0.0.1"},
	})
	ctx := lift.NewContext(context.Background(), req)

	key := generateKey(ctx)
	require.Equal(t, "ip:10.0.0.1", key.Identifier)
	require.Equal(t, "10.0.0.1", key.Metadata["ip"])

	ctx.Request.Headers = map[string]string{"X-Real-IP": "10.0.0.2"}
	key = generateKey(ctx)
	require.Equal(t, "ip:10.0.0.2", key.Identifier)

	ctx.Request.Headers = map[string]string{}
	key = generateKey(ctx)
	require.Equal(t, "ip:unknown", key.Identifier)

	ctx.SetTenantID("tenant-1")
	key = generateKey(ctx)
	require.Equal(t, "tenant:tenant-1", key.Identifier)
	require.Equal(t, "tenant-1", key.Metadata["tenant_id"])

	ctx.SetUserID("user-1")
	key = generateKey(ctx)
	require.Equal(t, "user:user-1", key.Identifier)
	require.Equal(t, "user-1", key.Metadata["user_id"])
}

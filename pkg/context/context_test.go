package context

import (
	"context"
	"testing"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestContextWrapper_Basics(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{
		Method:      "GET",
		Path:        "/users/123",
		QueryParams: map[string]string{"q": "v"},
		PathParams:  map[string]string{"id": "123"},
	})
	base := lift.NewContext(context.Background(), req)
	base.SetTenantID("tenant-1")
	base.SetUserID("user-1")

	ctx := NewContext(base)
	require.Same(t, base, ctx.Context)
	require.Equal(t, base.Context, ctx.GoContext())
	require.Equal(t, "tenant-1", ctx.TenantID())
	require.Equal(t, "user-1", ctx.UserID())
}

func TestContextWrapper_Logger(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/"})
	base := lift.NewContext(context.Background(), req)

	// Fallback path.
	got := NewContext(base).Logger()
	require.NotNil(t, got)
	require.IsType(t, &noOpLogger{}, got)

	// Provided logger path.
	expected := &noOpLogger{}
	base.Set("logger", expected)
	got = NewContext(base).Logger()
	require.Same(t, expected, got)
}

func TestNoOpLogger(t *testing.T) {
	l := &noOpLogger{}

	l.Debug("d")
	l.Info("i")
	l.Warn("w")
	l.Error("e")
	l.Fatal("f")

	require.Same(t, l, l.WithField("k", "v"))
	require.Same(t, l, l.WithFields(map[string]any{"k": "v"}))

	require.Same(t, l, l.WithRequestID("r"))
	require.Same(t, l, l.WithTenantID("t"))
	require.Same(t, l, l.WithUserID("u"))
	require.Same(t, l, l.WithTraceID("tr"))
	require.Same(t, l, l.WithSpanID("sp"))

	require.NoError(t, l.Flush(context.Background()))
	require.NoError(t, l.Close())
	require.True(t, l.IsHealthy())
	require.NotNil(t, l.GetStats())
}

func TestContextWrapper_ParamsAndParsing(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{
		Method:      "POST",
		Path:        "/things/abc",
		QueryParams: map[string]string{"present": "yes"},
		Headers:     map[string]string{"Content-Type": "application/json"},
		Body:        []byte(`{"name":"n"}`),
	})
	base := lift.NewContext(context.Background(), req)
	base.SetParam("id", "abc")

	ctx := NewContext(base)

	require.Equal(t, "abc", ctx.PathParam("id"))
	require.Equal(t, "yes", ctx.QueryParam("present"))
	require.Equal(t, "default", ctx.QueryParam("missing", "default"))

	type payload struct {
		Name string `json:"name"`
	}
	var p payload
	require.NoError(t, ctx.ParseJSON(&p))
	require.Equal(t, "n", p.Name)
}

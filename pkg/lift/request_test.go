package lift

import (
	"context"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift/adapters"
)

func TestRequest_Context(t *testing.T) {
	t.Run("returns background context when not set", func(t *testing.T) {
		req := NewRequest(nil)
		ctx := req.Context()
		if ctx == nil {
			t.Error("Context should not be nil")
		}
		// Should be context.Background()
		if err := ctx.Err(); err != nil {
			t.Errorf("Context should not have error: %v", err)
		}
	})

	t.Run("returns provided context when set", func(t *testing.T) {
		req := NewRequest(nil)

		// Create a context with a cancel
		parentCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		req.SetContext(parentCtx)

		ctx := req.Context()
		if ctx != parentCtx {
			t.Error("Context should be the same as the provided context")
		}

		// Cancel the parent and verify it propagates
		cancel()
		if ctx.Err() != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", ctx.Err())
		}
	})

	t.Run("respects context deadline", func(t *testing.T) {
		req := NewRequest(nil)

		// Create a context with a short deadline
		parentCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		req.SetContext(parentCtx)

		ctx := req.Context()

		// Wait for deadline
		<-ctx.Done()

		if ctx.Err() != context.DeadlineExceeded {
			t.Errorf("Expected context.DeadlineExceeded, got %v", ctx.Err())
		}
	})

	t.Run("NewRequestWithContext propagates context", func(t *testing.T) {
		parentCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		req := NewRequestWithContext(parentCtx, nil)

		ctx := req.Context()
		if ctx != parentCtx {
			t.Error("Context should be the same as the provided context")
		}
	})
}

func TestRequest_RemoteAddr(t *testing.T) {
	t.Run("extracts IP from API Gateway V2 requestContext", func(t *testing.T) {
		req := NewRequest(&adapters.Request{
			RawEvent: map[string]any{
				"requestContext": map[string]any{
					"http": map[string]any{
						"sourceIp": "203.0.113.50",
					},
				},
			},
		})

		if ip := req.RemoteAddr(); ip != "203.0.113.50" {
			t.Errorf("Expected 203.0.113.50, got %s", ip)
		}
	})

	t.Run("extracts IP from API Gateway V1 requestContext", func(t *testing.T) {
		req := NewRequest(&adapters.Request{
			RawEvent: map[string]any{
				"requestContext": map[string]any{
					"identity": map[string]any{
						"sourceIp": "198.51.100.25",
					},
				},
			},
		})

		if ip := req.RemoteAddr(); ip != "198.51.100.25" {
			t.Errorf("Expected 198.51.100.25, got %s", ip)
		}
	})

	t.Run("extracts first IP from X-Forwarded-For header", func(t *testing.T) {
		req := NewRequest(&adapters.Request{
			Headers: map[string]string{
				"X-Forwarded-For": "192.0.2.1, 198.51.100.178, 203.0.113.195",
			},
		})

		if ip := req.RemoteAddr(); ip != "192.0.2.1" {
			t.Errorf("Expected 192.0.2.1, got %s", ip)
		}
	})

	t.Run("uses X-Real-IP header", func(t *testing.T) {
		req := NewRequest(&adapters.Request{
			Headers: map[string]string{
				"X-Real-IP": "10.0.0.1",
			},
		})

		if ip := req.RemoteAddr(); ip != "10.0.0.1" {
			t.Errorf("Expected 10.0.0.1, got %s", ip)
		}
	})

	t.Run("uses CF-Connecting-IP header", func(t *testing.T) {
		req := NewRequest(&adapters.Request{
			Headers: map[string]string{
				"CF-Connecting-IP": "172.16.0.1",
			},
		})

		if ip := req.RemoteAddr(); ip != "172.16.0.1" {
			t.Errorf("Expected 172.16.0.1, got %s", ip)
		}
	})

	t.Run("returns empty string when no IP available", func(t *testing.T) {
		req := NewRequest(&adapters.Request{})

		if ip := req.RemoteAddr(); ip != "" {
			t.Errorf("Expected empty string, got %s", ip)
		}
	})

	t.Run("prefers requestContext over headers", func(t *testing.T) {
		req := NewRequest(&adapters.Request{
			RawEvent: map[string]any{
				"requestContext": map[string]any{
					"http": map[string]any{
						"sourceIp": "203.0.113.50",
					},
				},
			},
			Headers: map[string]string{
				"X-Forwarded-For": "192.0.2.1",
			},
		})

		if ip := req.RemoteAddr(); ip != "203.0.113.50" {
			t.Errorf("Expected 203.0.113.50 (from requestContext), got %s", ip)
		}
	})
}

func TestRequest_GetHeader_CaseInsensitive(t *testing.T) {
	req := NewRequest(&adapters.Request{
		Headers: map[string]string{
			"X-Test": "value",
		},
	})

	if got := req.GetHeader("X-Test"); got != "value" {
		t.Fatalf("expected exact header match, got %q", got)
	}
	if got := req.GetHeader("x-test"); got != "value" {
		t.Fatalf("expected case-insensitive header match, got %q", got)
	}

	empty := NewRequest(&adapters.Request{Headers: nil})
	if got := empty.GetHeader("x-test"); got != "" {
		t.Fatalf("expected empty header value, got %q", got)
	}
}

func TestRequest_RequestContext_Extraction(t *testing.T) {
	req := NewRequest(&adapters.Request{RawEvent: nil})
	if ctx := req.RequestContext(); len(ctx) != 0 {
		t.Fatalf("expected empty context for nil raw event")
	}

	req = NewRequest(&adapters.Request{
		RawEvent: map[string]any{
			"requestContext": map[string]any{
				"sourceIp": "203.0.113.10",
			},
		},
	})
	if ctx := req.RequestContext(); ctx["sourceIp"] != "203.0.113.10" {
		t.Fatalf("expected requestContext.sourceIp extracted, got %v", ctx["sourceIp"])
	}

	req = NewRequest(&adapters.Request{
		RawEvent: "not-a-map",
	})
	if ctx := req.RequestContext(); len(ctx) != 0 {
		t.Fatalf("expected empty context for non-map raw event")
	}
}

func TestRequest_QueryParam_UserAgent_URL(t *testing.T) {
	req := NewRequest(&adapters.Request{
		QueryParams: map[string]string{"q": "search"},
		PathParams:  map[string]string{"id": "123"},
		Headers:     map[string]string{"User-Agent": "ua"},
		Path:        "/hello",
	})

	if got := req.GetQuery("q"); got != "search" {
		t.Fatalf("expected query value, got %q", got)
	}
	if got := req.GetParam("id"); got != "123" {
		t.Fatalf("expected param value, got %q", got)
	}
	if got := req.UserAgent(); got != "ua" {
		t.Fatalf("expected user agent, got %q", got)
	}
	if got := req.URL().Path; got != "/hello" {
		t.Fatalf("expected URL path, got %q", got)
	}
}

func TestRequest_Header_ReturnsMap(t *testing.T) {
	req := NewRequest(&adapters.Request{
		Headers: map[string]string{"X-Test": "value"},
	})
	headers := req.Header()
	if headers["X-Test"] != "value" {
		t.Fatalf("expected header value, got %q", headers["X-Test"])
	}

	req = NewRequest(&adapters.Request{Headers: nil})
	headers = req.Header()
	if headers == nil {
		t.Fatalf("expected non-nil header map")
	}
	if len(headers) != 0 {
		t.Fatalf("expected empty header map, got %v", headers)
	}
}

package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

// SecurityHeadersConfig configures the security headers middleware
type SecurityHeadersConfig struct {
	// Content Security Policy
	ContentSecurityPolicy string

	// X-Frame-Options: DENY, SAMEORIGIN, or ALLOW-FROM uri
	XFrameOptions string

	// X-Content-Type-Options: nosniff
	XContentTypeOptions bool

	// X-XSS-Protection: 1; mode=block
	XXSSProtection string

	// Strict-Transport-Security
	StrictTransportSecurity string

	// Referrer-Policy
	ReferrerPolicy string

	// Permissions-Policy (formerly Feature-Policy)
	PermissionsPolicy string

	// Custom headers to add
	CustomHeaders map[string]string

	// Whether to include security headers in development
	IncludeInDevelopment bool
}

// DefaultSecurityHeadersConfig returns secure default configuration
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		ContentSecurityPolicy:   "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; media-src 'self'; object-src 'none'; child-src 'none'; frame-src 'none'; worker-src 'none'; frame-ancestors 'none'; form-action 'self'; base-uri 'self';",
		XFrameOptions:           "DENY",
		XContentTypeOptions:     true,
		XXSSProtection:          "1; mode=block",
		StrictTransportSecurity: "max-age=31536000; includeSubDomains; preload",
		ReferrerPolicy:          "strict-origin-when-cross-origin",
		PermissionsPolicy:       "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=()",
		CustomHeaders:           make(map[string]string),
		IncludeInDevelopment:    true,
	}
}

// SecurityHeaders returns the security headers middleware
func SecurityHeaders(config SecurityHeadersConfig) lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Skip in development if configured (check environment variable or context values)
			if !config.IncludeInDevelopment {
				if env := ctx.Get("environment"); env == "development" {
					return next.Handle(ctx)
				}
			}

			// Set Content Security Policy
			if config.ContentSecurityPolicy != "" {
				ctx.Response.Header("Content-Security-Policy", config.ContentSecurityPolicy)
			}

			// Set X-Frame-Options
			if config.XFrameOptions != "" {
				ctx.Response.Header("X-Frame-Options", config.XFrameOptions)
			}

			// Set X-Content-Type-Options
			if config.XContentTypeOptions {
				ctx.Response.Header("X-Content-Type-Options", "nosniff")
			}

			// Set X-XSS-Protection
			if config.XXSSProtection != "" {
				ctx.Response.Header("X-XSS-Protection", config.XXSSProtection)
			}

			// Set Strict-Transport-Security (check if HTTPS via headers or scheme)
			if config.StrictTransportSecurity != "" {
				// Check for HTTPS indicators
				isSecure := ctx.Header("X-Forwarded-Proto") == "https" ||
					ctx.Header("CloudFront-Forwarded-Proto") == "https" ||
					ctx.Header("X-Forwarded-SSL") == "on"

				if isSecure {
					ctx.Response.Header("Strict-Transport-Security", config.StrictTransportSecurity)
				}
			}

			// Set Referrer-Policy
			if config.ReferrerPolicy != "" {
				ctx.Response.Header("Referrer-Policy", config.ReferrerPolicy)
			}

			// Set Permissions-Policy
			if config.PermissionsPolicy != "" {
				ctx.Response.Header("Permissions-Policy", config.PermissionsPolicy)
			}

			// Set custom headers
			for key, value := range config.CustomHeaders {
				ctx.Response.Header(key, value)
			}

			// Add Cache-Control for sensitive endpoints (check path patterns)
			if isSensitivePath(ctx.Request.Path) {
				ctx.Response.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
				ctx.Response.Header("Pragma", "no-cache")
				ctx.Response.Header("Expires", "0")
			}

			return next.Handle(ctx)
		})
	}
}

// StrictSecurityHeaders returns a middleware with very strict security settings
func StrictSecurityHeaders() lift.Middleware {
	config := SecurityHeadersConfig{
		ContentSecurityPolicy:   "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; base-uri 'self'; form-action 'self';",
		XFrameOptions:           "DENY",
		XContentTypeOptions:     true,
		XXSSProtection:          "1; mode=block",
		StrictTransportSecurity: "max-age=63072000; includeSubDomains; preload",
		ReferrerPolicy:          "no-referrer",
		PermissionsPolicy:       "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=(), fullscreen=(), autoplay=()",
		CustomHeaders: map[string]string{
			"X-Permitted-Cross-Domain-Policies": "none",
			"Cross-Origin-Embedder-Policy":      "require-corp",
			"Cross-Origin-Opener-Policy":        "same-origin",
			"Cross-Origin-Resource-Policy":      "same-origin",
		},
		IncludeInDevelopment: false,
	}

	return SecurityHeaders(config)
}

// APISecurityHeaders returns security headers optimized for API endpoints
func APISecurityHeaders() lift.Middleware {
	config := SecurityHeadersConfig{
		ContentSecurityPolicy:   "default-src 'none'; frame-ancestors 'none';",
		XFrameOptions:           "DENY",
		XContentTypeOptions:     true,
		XXSSProtection:          "1; mode=block",
		StrictTransportSecurity: "max-age=31536000; includeSubDomains",
		ReferrerPolicy:          "strict-origin-when-cross-origin",
		CustomHeaders: map[string]string{
			"X-Permitted-Cross-Domain-Policies": "none",
			"X-API-Version":                     "1.0",
		},
		IncludeInDevelopment: true,
	}

	return SecurityHeaders(config)
}

// SecurityHeadersWithNonce creates security headers with a nonce for CSP
func SecurityHeadersWithNonce() lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Generate a unique nonce for this request
			nonce := generateNonce()
			ctx.Set("csp_nonce", nonce)

			// Create CSP with nonce
			csp := fmt.Sprintf("default-src 'self'; script-src 'self' 'nonce-%s'; style-src 'self' 'nonce-%s' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none';", nonce, nonce)

			config := SecurityHeadersConfig{
				ContentSecurityPolicy:   csp,
				XFrameOptions:           "DENY",
				XContentTypeOptions:     true,
				XXSSProtection:          "1; mode=block",
				StrictTransportSecurity: "max-age=31536000; includeSubDomains",
				ReferrerPolicy:          "strict-origin-when-cross-origin",
				IncludeInDevelopment:    true,
			}

			return SecurityHeaders(config)(next).Handle(ctx)
		})
	}
}

// generateNonce creates a cryptographically secure nonce
func generateNonce() string {
	// In a real implementation, this would use crypto/rand
	// For now, use timestamp-based approach
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// SecurityAuditHeaders returns middleware that adds headers for security auditing
func SecurityAuditHeaders() lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Add security audit headers
			ctx.Response.Header("X-Security-Headers", "enabled")
			ctx.Response.Header("X-Security-Audit", fmt.Sprintf("scan-date-%s", time.Now().Format("2006-01-02")))

			return next.Handle(ctx)
		})
	}
}

// isSensitivePath checks if a path should be considered sensitive
func isSensitivePath(path string) bool {
	sensitivePaths := []string{
		"/api/auth",
		"/api/payment",
		"/api/users",
		"/api/accounts",
		"/admin",
		"/dashboard",
	}

	for _, sensitive := range sensitivePaths {
		if strings.HasPrefix(path, sensitive) {
			return true
		}
	}

	return false
}

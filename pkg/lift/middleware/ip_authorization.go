package middleware

import (
	"fmt"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/security"
)

// IPAuthorizationConfig holds configuration for IP authorization middleware
type IPAuthorizationConfig struct {
	IPAuthService *security.IPAuthorizationService
}

// IPAuthorization creates a middleware that checks if the request's source IP is authorized
func IPAuthorization(config IPAuthorizationConfig) lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Extract source IP
			sourceIP, err := security.ExtractClientIP(ctx.Request.Headers, ctx.Request.RequestContext())
			if err != nil {
				return ctx.BadRequest("Unable to determine source IP", err)
			}

			// Check if the source IP is authorized
			authorized, err := config.IPAuthService.IsAuthorizedIP(ctx.Context, sourceIP)
			if err != nil {
				return ctx.JSON(map[string]string{
					"error": "Failed to check IP authorization",
				})
			}

			if !authorized {
				return ctx.Forbidden("Unauthorized IP address", fmt.Errorf("IP not authorized: %s", sourceIP))
			}

			// IP is authorized, proceed to the next handler
			return next.Handle(ctx)
		})
	}
}

// CheckIPAuthorization is a helper function that performs IP authorization check
// It can be used within handlers when middleware approach is not suitable
func CheckIPAuthorization(ctx *lift.Context, ipAuthService *security.IPAuthorizationService) error {
	ctx.Logger.Info("CheckIPAuthorization start", map[string]any{
		"responseWritten": ctx.Response != nil && ctx.Response.IsWritten(),
	})
	
	// Extract source IP
	sourceIP, err := security.ExtractClientIP(ctx.Request.Headers, ctx.Request.RequestContext())
	if err != nil {
		ctx.Logger.Info("CheckIPAuthorization ExtractClientIP failed", map[string]any{
			"error": err.Error(),
			"responseWritten": ctx.Response != nil && ctx.Response.IsWritten(),
		})
		return ctx.BadRequest("Unable to determine source IP", err)
	}

	ctx.Logger.Info("CheckIPAuthorization ExtractClientIP success", map[string]any{
		"sourceIP": sourceIP,
		"responseWritten": ctx.Response != nil && ctx.Response.IsWritten(),
	})

	// Check if the source IP is authorized
	authorized, err := ipAuthService.IsAuthorizedIP(ctx.Context, sourceIP)
	if err != nil {
		ctx.Logger.Info("CheckIPAuthorization IsAuthorizedIP failed", map[string]any{
			"error": err.Error(),
			"responseWritten": ctx.Response != nil && ctx.Response.IsWritten(),
		})
		return ctx.SystemError("Failed to check IP authorization", err)
	}

	ctx.Logger.Info("CheckIPAuthorization IsAuthorizedIP success", map[string]any{
		"authorized": authorized,
		"responseWritten": ctx.Response != nil && ctx.Response.IsWritten(),
	})

	if !authorized {
		ctx.Logger.Info("CheckIPAuthorization unauthorized", map[string]any{
			"sourceIP": sourceIP,
			"responseWritten": ctx.Response != nil && ctx.Response.IsWritten(),
		})
		return ctx.Forbidden("Unauthorized IP address", fmt.Errorf("IP not authorized: %s", sourceIP))
	}

	ctx.Logger.Info("CheckIPAuthorization end success", map[string]any{
		"responseWritten": ctx.Response != nil && ctx.Response.IsWritten(),
	})
	return nil
}

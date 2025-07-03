package security

import (
	"fmt"
	"net"
	"strings"
)

// IPExtractionError represents an error during IP extraction
type IPExtractionError struct {
	Message string
	Headers map[string]string
}

func (e *IPExtractionError) Error() string {
	return fmt.Sprintf("failed to extract client IP: %s", e.Message)
}

// ExtractClientIP extracts the client's source IP address from various headers and request context.
// It follows the precedence order commonly used in production environments:
// 1. X-Forwarded-For (first IP in comma-separated list)
// 2. X-Real-IP
// 3. CF-Connecting-IP (Cloudflare)
// 4. X-Original-Forwarded-For
// 5. Request context (API Gateway specific)
//
// Returns an error if no valid IP address can be extracted.
func ExtractClientIP(headers map[string]string, requestContext map[string]any) (string, error) {
	// Collect relevant headers for error reporting
	relevantHeaders := make(map[string]string)
	
	// Try X-Forwarded-For header first (most common for load balancers)
	if forwardedFor, ok := headers["X-Forwarded-For"]; ok && forwardedFor != "" {
		relevantHeaders["X-Forwarded-For"] = forwardedFor
		// Take the first IP in the chain (original client IP)
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			sourceIP := strings.TrimSpace(ips[0])
			if isValidIP(sourceIP) {
				return stripPort(sourceIP), nil
			}
		}
	}

	// Try X-Real-IP header
	if xRealIP, ok := headers["X-Real-IP"]; ok && xRealIP != "" {
		relevantHeaders["X-Real-IP"] = xRealIP
		if isValidIP(xRealIP) {
			return stripPort(xRealIP), nil
		}
	}

	// Try Cloudflare-specific header
	if cfIP, ok := headers["CF-Connecting-IP"]; ok && cfIP != "" {
		relevantHeaders["CF-Connecting-IP"] = cfIP
		if isValidIP(cfIP) {
			return stripPort(cfIP), nil
		}
	}

	// Try X-Original-Forwarded-For (some proxies use this)
	if origForwarded, ok := headers["X-Original-Forwarded-For"]; ok && origForwarded != "" {
		relevantHeaders["X-Original-Forwarded-For"] = origForwarded
		ips := strings.Split(origForwarded, ",")
		if len(ips) > 0 {
			sourceIP := strings.TrimSpace(ips[0])
			if isValidIP(sourceIP) {
				return stripPort(sourceIP), nil
			}
		}
	}

	// Try to extract from request context (API Gateway specific)
	if requestContext != nil {
		// API Gateway v2 format
		if httpContext, ok := requestContext["http"].(map[string]any); ok {
			if sourceIP, ok := httpContext["sourceIp"].(string); ok && sourceIP != "" {
				if isValidIP(sourceIP) {
					return stripPort(sourceIP), nil
				}
				relevantHeaders["requestContext.http.sourceIp"] = sourceIP
			}
		}

		// API Gateway v1 format
		if identity, ok := requestContext["identity"].(map[string]any); ok {
			if sourceIP, ok := identity["sourceIp"].(string); ok && sourceIP != "" {
				if isValidIP(sourceIP) {
					return stripPort(sourceIP), nil
				}
				relevantHeaders["requestContext.identity.sourceIp"] = sourceIP
			}
		}

		// Direct sourceIp field (some Lambda integrations)
		if sourceIP, ok := requestContext["sourceIp"].(string); ok && sourceIP != "" {
			if isValidIP(sourceIP) {
				return stripPort(sourceIP), nil
			}
			relevantHeaders["requestContext.sourceIp"] = sourceIP
		}
	}

	// Return error with context about what was checked
	return "", &IPExtractionError{
		Message: "no valid IP address found in headers or request context",
		Headers: relevantHeaders,
	}
}

// stripPort removes port from IP address if present
func stripPort(ip string) string {
	// Handle IPv4 with port
	if strings.Contains(ip, ":") && !strings.Contains(ip, "::") {
		host, _, err := net.SplitHostPort(ip)
		if err == nil {
			return host
		}
	}
	return ip
}

// isValidIP checks if the given string is a valid IP address
func isValidIP(ip string) bool {
	// Remove port if present
	if strings.Contains(ip, ":") && !strings.Contains(ip, "::") {
		// IPv4 with port
		host, _, err := net.SplitHostPort(ip)
		if err == nil {
			ip = host
		}
	}
	
	return net.ParseIP(ip) != nil
}
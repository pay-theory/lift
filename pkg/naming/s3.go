package naming

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const defaultBucketName = "bucket"

// SanitizeS3BucketName converts an arbitrary name into a valid S3 bucket name.
//
// Lift intentionally restricts output to lowercase letters, numbers, and hyphens
// to avoid dot-related edge cases (e.g., TLS wildcard and legacy behavior).
func SanitizeS3BucketName(name string) string {
	raw := strings.ToLower(strings.TrimSpace(name))
	if raw == "" {
		raw = defaultBucketName
	}

	out := make([]byte, 0, len(raw))
	lastDash := false
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out = append(out, byte(r))
			lastDash = false
			continue
		}

		if !lastDash {
			out = append(out, '-')
			lastDash = true
		}
	}

	safe := strings.Trim(string(out), "-")
	if safe == "" {
		safe = defaultBucketName
	}
	if len(safe) < 3 {
		safe += "-bucket"
	}

	if len(safe) > 63 {
		sum := sha256.Sum256([]byte(raw))
		suffix := hex.EncodeToString(sum[:])[:8]

		// Leave room for "-<hash>".
		maxBase := 63 - 1 - len(suffix)
		if maxBase < 1 {
			maxBase = 1
		}
		base := strings.Trim(safe[:maxBase], "-")
		if base == "" {
			base = defaultBucketName
		}
		safe = base + "-" + suffix
	}

	safe = strings.Trim(safe, "-")
	if len(safe) < 3 {
		// Worst-case: if trimming removed everything, fall back to a stable name.
		sum := sha256.Sum256([]byte(raw))
		safe = "bucket-" + hex.EncodeToString(sum[:])[:8]
	}

	return safe
}

// S3BucketName returns a deterministic, S3-safe bucket name derived from the naming context.
func (c Context) S3BucketName(resource string) string {
	return SanitizeS3BucketName(c.ResourceName(resource))
}

// S3BucketNameFromEnv returns a bucket name derived from env vars, or ("", false) if incomplete.
func S3BucketNameFromEnv(resource string) (string, bool) {
	ctx := FromEnv()
	if !ctx.IsComplete() {
		return "", false
	}
	return ctx.S3BucketName(resource), true
}

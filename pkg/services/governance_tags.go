package services

import "strings"

const (
	GovernanceTagTenantPrefix  = "tenant:"
	GovernanceTagUserPrefix    = "user:"
	GovernanceTagFeaturePrefix = "feature:"
)

func GovernanceTagTenant(tenantID string) string {
	return governanceTag(GovernanceTagTenantPrefix, tenantID)
}

func GovernanceTagUser(userID string) string {
	return governanceTag(GovernanceTagUserPrefix, userID)
}

func GovernanceTagFeature(feature string) string {
	feature = strings.TrimSpace(feature)
	if feature == "" {
		return ""
	}
	return GovernanceTagFeaturePrefix + strings.ToLower(feature)
}

// ApplyGovernanceTags appends normalized governance tags and de-duplicates event.Tags in-place.
//
// This is intended for consistent tagging across services so cost attribution and throttling can
// be layered on top of EventBus cleanly.
func ApplyGovernanceTags(event *Event, tenantID string, userID string, feature string) {
	if event == nil {
		return
	}

	tags := make([]string, 0, len(event.Tags)+3)
	tags = append(tags, event.Tags...)
	tags = append(tags,
		GovernanceTagTenant(tenantID),
		GovernanceTagUser(userID),
		GovernanceTagFeature(feature),
	)

	event.Tags = DeduplicateTags(tags)
}

// DeduplicateTags trims, removes empty entries, and de-duplicates tags preserving first-seen order.
func DeduplicateTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}

	return out
}

func governanceTag(prefix string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return prefix + value
}

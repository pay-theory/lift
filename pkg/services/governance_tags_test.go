package services

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGovernanceTags_FormattingAndDeduplication(t *testing.T) {
	require.Equal(t, "tenant:tenant-1", GovernanceTagTenant("tenant-1"))
	require.Equal(t, "tenant:tenant-1", GovernanceTagTenant(" tenant-1 "))
	require.Equal(t, "", GovernanceTagTenant("   "))

	require.Equal(t, "user:user-1", GovernanceTagUser("user-1"))
	require.Equal(t, "", GovernanceTagUser(""))

	require.Equal(t, "feature:checkout", GovernanceTagFeature("Checkout"))
	require.Equal(t, "feature:checkout", GovernanceTagFeature("  CHECKOUT  "))
	require.Equal(t, "", GovernanceTagFeature("   "))

	tags := DeduplicateTags([]string{" a ", "b", "a", "", "   ", "b", "c"})
	require.Equal(t, []string{"a", "b", "c"}, tags)

	event := &Event{Tags: []string{"existing", "tenant:tenant-1"}}
	ApplyGovernanceTags(event, "tenant-1", "user-1", "Feature")
	require.ElementsMatch(t, []string{
		"existing",
		"tenant:tenant-1",
		"user:user-1",
		"feature:feature",
	}, event.Tags)
}

func TestGovernanceTags_NilEventAndEmptyTags(t *testing.T) {
	ApplyGovernanceTags(nil, "tenant-1", "user-1", "Feature")
	require.Nil(t, DeduplicateTags(nil))
	require.Nil(t, DeduplicateTags([]string{}))
}

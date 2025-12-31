package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDeployFlags(t *testing.T) {
	stage, partner, targetMode := parseDeployFlags(nil)
	require.Equal(t, "dev", stage)
	require.Empty(t, partner)
	require.Empty(t, targetMode)

	stage, partner, targetMode = parseDeployFlags([]string{"--stage", "staging", "--partner", "p1", "--target-mode", "tm"})
	require.Equal(t, "staging", stage)
	require.Equal(t, "p1", partner)
	require.Equal(t, "tm", targetMode)

	stage, partner, targetMode = parseDeployFlags([]string{"--stage=live", "--partner=p2", "--target-mode=custom"})
	require.Equal(t, "live", stage)
	require.Equal(t, "p2", partner)
	require.Equal(t, "custom", targetMode)
}

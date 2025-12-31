package cli

import (
	"testing"

	"github.com/pay-theory/lift/internal/liftconfig"
	"github.com/stretchr/testify/require"
)

func TestResolveDeployContext(t *testing.T) {
	cfgUsesPartner := &liftconfig.Config{
		CDK: &liftconfig.CDKConfig{
			Stacks: map[string]*liftconfig.Stack{
				"service": {NameTemplate: "{{.AppName}}-{{.Partner}}-{{.Stage}}"},
			},
		},
	}

	_, _, err := resolveDeployContext(cfgUsesPartner, "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--partner is required")

	partner, targetMode, err := resolveDeployContext(cfgUsesPartner, "p1", "")
	require.NoError(t, err)
	require.Equal(t, "p1", partner)
	require.Equal(t, defaultTargetMode, targetMode)

	cfgUsesTargetMode := &liftconfig.Config{
		CDK: &liftconfig.CDKConfig{
			Stacks: map[string]*liftconfig.Stack{
				"service": {NameTemplate: "{{.AppName}}-{{.TargetMode}}-{{.Stage}}"},
			},
		},
	}

	partner, targetMode, err = resolveDeployContext(cfgUsesTargetMode, "", "")
	require.NoError(t, err)
	require.Empty(t, partner)
	require.Equal(t, defaultTargetMode, targetMode)

	partner, targetMode, err = resolveDeployContext(cfgUsesTargetMode, "", "explicit")
	require.NoError(t, err)
	require.Empty(t, partner)
	require.Equal(t, "explicit", targetMode)
}

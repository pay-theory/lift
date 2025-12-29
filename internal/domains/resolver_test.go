package domains

import (
	"testing"

	"github.com/pay-theory/lift/internal/liftconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateStage(t *testing.T) {
	tests := []struct {
		name    string
		stage   string
		wantErr bool
	}{
		{"dev is valid", "dev", false},
		{"staging is valid", "staging", false},
		{"live is valid", "live", false},
		{"empty is invalid", "", true},
		{"prod is invalid", "prod", true},
		{"production is invalid", "production", true},
		{"test is invalid", "test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStage(tt.stage)
			if tt.wantErr {
				require.Error(t, err)
				var stageErr *InvalidStageError
				assert.ErrorAs(t, err, &stageErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDomainsEnabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  *liftconfig.Config
		want bool
	}{
		{
			name: "enabled with base domain",
			cfg: &liftconfig.Config{
				Domains: &liftconfig.Domains{BaseDomain: "example.com"},
			},
			want: true,
		},
		{
			name: "disabled when domains is nil",
			cfg:  &liftconfig.Config{},
			want: false,
		},
		{
			name: "disabled when base domain is empty",
			cfg: &liftconfig.Config{
				Domains: &liftconfig.Domains{BaseDomain: ""},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DomainsEnabled(tt.cfg))
		})
	}
}

func TestResolve_DefaultDomains(t *testing.T) {
	cfg := &liftconfig.Config{
		Domains: &liftconfig.Domains{BaseDomain: "example.com"},
		Services: map[string]*liftconfig.Service{
			"api": {Subdomain: "api"},
		},
	}

	tests := []struct {
		stage         string
		wantStageRoot string
		wantAPIDomain string
	}{
		{"dev", "dev.example.com", "api.dev.example.com"},
		{"staging", "staging.example.com", "api.staging.example.com"},
		{"live", "example.com", "api.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.stage, func(t *testing.T) {
			resolved, err := Resolve(cfg, tt.stage)
			require.NoError(t, err)
			require.NotNil(t, resolved)

			assert.Equal(t, "example.com", resolved.BaseDomain)
			assert.Equal(t, tt.wantStageRoot, resolved.StageRootDomain)
			assert.Equal(t, tt.wantAPIDomain, resolved.Services["api"])
		})
	}
}

func TestResolve_ExplicitStageRootDomain(t *testing.T) {
	cfg := &liftconfig.Config{
		Domains: &liftconfig.Domains{BaseDomain: "example.com"},
		Stages: liftconfig.StageMap{
			"dev": {RootDomain: "custom-dev.example.com"},
		},
		Services: map[string]*liftconfig.Service{
			"api": {Subdomain: "api"},
		},
	}

	resolved, err := Resolve(cfg, "dev")
	require.NoError(t, err)
	require.NotNil(t, resolved)

	assert.Equal(t, "custom-dev.example.com", resolved.StageRootDomain)
	assert.Equal(t, "api.custom-dev.example.com", resolved.Services["api"])
}

func TestResolve_MultipleServices(t *testing.T) {
	cfg := &liftconfig.Config{
		Domains: &liftconfig.Domains{BaseDomain: "example.com"},
		Services: map[string]*liftconfig.Service{
			"api":    {Subdomain: "api"},
			"admin":  {Subdomain: "admin"},
			"worker": {}, // no subdomain
		},
	}

	resolved, err := Resolve(cfg, "dev")
	require.NoError(t, err)
	require.NotNil(t, resolved)

	assert.Equal(t, "api.dev.example.com", resolved.Services["api"])
	assert.Equal(t, "admin.dev.example.com", resolved.Services["admin"])
	assert.Empty(t, resolved.Services["worker"]) // no subdomain configured
}

func TestResolve_NoDomains(t *testing.T) {
	cfg := &liftconfig.Config{
		App: liftconfig.AppConfig{Name: "test-app"},
	}

	resolved, err := Resolve(cfg, "dev")
	require.NoError(t, err)
	assert.Nil(t, resolved)
}

func TestResolve_InvalidStage(t *testing.T) {
	cfg := &liftconfig.Config{
		Domains: &liftconfig.Domains{BaseDomain: "example.com"},
	}

	_, err := Resolve(cfg, "production")
	require.Error(t, err)
	var stageErr *InvalidStageError
	assert.ErrorAs(t, err, &stageErr)
}

func TestResolve_EmptyServices(t *testing.T) {
	cfg := &liftconfig.Config{
		Domains: &liftconfig.Domains{BaseDomain: "example.com"},
	}

	resolved, err := Resolve(cfg, "dev")
	require.NoError(t, err)
	require.NotNil(t, resolved)

	assert.Equal(t, "dev.example.com", resolved.StageRootDomain)
	assert.Empty(t, resolved.Services)
}

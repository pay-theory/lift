package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCommandV2_ParseArgs_UnknownFlagAndUnexpectedArg(t *testing.T) {
	cmd := &NewCommandV2{}

	_, err := cmd.parseArgs([]string{"--nope"})
	require.Error(t, err)

	_, err = cmd.parseArgs([]string{"app1", "app2"})
	require.Error(t, err)
}

func TestNewCommandV2_ValidateOpts_UnknownTemplate(t *testing.T) {
	cmd := &NewCommandV2{}

	err := cmd.validateOpts(&newOpts{
		template:   "does-not-exist",
		baseDomain: "example.com",
	})
	require.Error(t, err)
}

func TestNewCommandV2_ResolveTargetDir_PathExistsAsFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "my-app"), []byte("x"), 0600))

	cmd := &NewCommandV2{}
	_, err = cmd.resolveTargetDir("my-app")
	require.Error(t, err)
}

func TestNewCommandV2_RenderTemplate_ParseAndExecuteErrors(t *testing.T) {
	cmd := &NewCommandV2{}

	_, err := cmd.renderTemplate("{{.Bad", TemplateData{AppName: "app"})
	require.Error(t, err)

	_, err = cmd.renderTemplate(`{{template "missing" .}}`, TemplateData{AppName: "app"})
	require.Error(t, err)
}

func TestNewCommandV2_ScaffoldPTFiles_UnknownTemplate(t *testing.T) {
	cmd := &NewCommandV2{}
	err := cmd.scaffoldPTFiles(t.TempDir(), TemplateData{AppName: "app"}, "unknown-template")
	require.Error(t, err)
}

func TestNewCommandV2_ParseArgs_HelpAndEqualsForms(t *testing.T) {
	cmd := &NewCommandV2{}

	opts, err := cmd.parseArgs([]string{"--help"})
	require.NoError(t, err)
	require.Nil(t, opts)

	opts, err = cmd.parseArgs([]string{"app", "--template=microservice", "--base-domain=example.com"})
	require.NoError(t, err)
	require.NotNil(t, opts)
	require.Equal(t, "app", opts.appName)
	require.Equal(t, "microservice", opts.template)
	require.Equal(t, "example.com", opts.baseDomain)
}

func TestNewCommandV2_ValidateOpts_PTNotSupportedBranch(t *testing.T) {
	cmd := &NewCommandV2{}

	original, ok := ptTemplates["basic-api"]
	require.True(t, ok)

	delete(ptTemplates, "basic-api")
	t.Cleanup(func() {
		ptTemplates["basic-api"] = original
	})

	err := cmd.validateOpts(&newOpts{
		template:   "basic-api",
		baseDomain: "example.com",
		pt:         true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not available")
}

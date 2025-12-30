package yaml_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	t.Run("loads valid config file", func(t *testing.T) {
		t.Parallel()

		path := writeConfigFile(t, `
server: https://example.atlassian.net
project: TEST
`)
		cfg, err := yaml.LoadConfig(path)

		require.NoError(t, err)
		assert.Equal(t, "https://example.atlassian.net", cfg.Server)
		assert.Equal(t, "TEST", cfg.Project)
	})

	t.Run("returns error for nonexistent file", func(t *testing.T) {
		t.Parallel()

		_, err := yaml.LoadConfig("/nonexistent/path/to/config.yaml")

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("returns error for invalid YAML", func(t *testing.T) {
		t.Parallel()

		path := writeConfigFile(t, `
server: [invalid
project: TEST
`)
		_, err := yaml.LoadConfig(path)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
	})

	t.Run("returns error for unreadable file", func(t *testing.T) {
		t.Parallel()

		// Create a directory instead of a file - attempting to read it will fail
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		require.NoError(t, os.Mkdir(path, 0o755))

		_, err := yaml.LoadConfig(path)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
	})

	t.Run("returns error when server is missing", func(t *testing.T) {
		t.Parallel()

		path := writeConfigFile(t, `
project: TEST
`)
		_, err := yaml.LoadConfig(path)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "server")
	})

	t.Run("returns error when project is missing", func(t *testing.T) {
		t.Parallel()

		path := writeConfigFile(t, `
server: https://example.atlassian.net
`)
		_, err := yaml.LoadConfig(path)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "project")
	})
}

// writeConfigFile creates a temporary YAML config file and returns its path.
func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

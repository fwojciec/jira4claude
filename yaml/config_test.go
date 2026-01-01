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

func TestDiscoverConfig(t *testing.T) {
	t.Parallel()

	validConfig := `server: https://example.atlassian.net
project: TEST
`

	t.Run("returns local config when it exists", func(t *testing.T) {
		t.Parallel()

		workDir := t.TempDir()
		homeDir := t.TempDir()
		localPath := filepath.Join(workDir, ".jira4claude.yaml")
		require.NoError(t, os.WriteFile(localPath, []byte(validConfig), 0o644))

		path, err := yaml.DiscoverConfig(workDir, homeDir)

		require.NoError(t, err)
		assert.Equal(t, localPath, path)
	})

	t.Run("returns global config when local does not exist", func(t *testing.T) {
		t.Parallel()

		workDir := t.TempDir()
		homeDir := t.TempDir()
		globalPath := filepath.Join(homeDir, ".jira4claude.yaml")
		require.NoError(t, os.WriteFile(globalPath, []byte(validConfig), 0o644))

		path, err := yaml.DiscoverConfig(workDir, homeDir)

		require.NoError(t, err)
		assert.Equal(t, globalPath, path)
	})

	t.Run("returns error when no config exists with clear fix command", func(t *testing.T) {
		t.Parallel()

		workDir := t.TempDir()
		homeDir := t.TempDir()

		_, err := yaml.DiscoverConfig(workDir, homeDir)

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), ".jira4claude.yaml")
		// Should include clear fix command with correct binary name
		assert.Contains(t, err.Error(), "j4c init")
		assert.Contains(t, err.Error(), "--server")
		assert.Contains(t, err.Error(), "--project")
	})

	t.Run("prefers local config over global when both exist", func(t *testing.T) {
		t.Parallel()

		workDir := t.TempDir()
		homeDir := t.TempDir()
		localPath := filepath.Join(workDir, ".jira4claude.yaml")
		globalPath := filepath.Join(homeDir, ".jira4claude.yaml")
		require.NoError(t, os.WriteFile(localPath, []byte(validConfig), 0o644))
		require.NoError(t, os.WriteFile(globalPath, []byte(validConfig), 0o644))

		path, err := yaml.DiscoverConfig(workDir, homeDir)

		require.NoError(t, err)
		assert.Equal(t, localPath, path)
	})
}

func TestInit(t *testing.T) {
	t.Parallel()

	t.Run("creates config file with correct content", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		result, err := yaml.Init(dir, "https://example.atlassian.net", "TEST")

		require.NoError(t, err)
		assert.True(t, result.ConfigCreated)

		// Verify file contents
		content, err := os.ReadFile(filepath.Join(dir, ".jira4claude.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "server: https://example.atlassian.net")
		assert.Contains(t, string(content), "project: TEST")
	})

	t.Run("returns error if config already exists", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		configPath := filepath.Join(dir, ".jira4claude.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("existing"), 0o644))

		_, err := yaml.Init(dir, "https://example.atlassian.net", "TEST")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("creates gitignore if missing", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		// No .gitignore exists in fresh temp dir

		result, err := yaml.Init(dir, "https://example.atlassian.net", "TEST")

		require.NoError(t, err)
		assert.True(t, result.GitignoreAdded)

		// Verify .gitignore was created with only our entry
		content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
		require.NoError(t, err)
		assert.Equal(t, ".jira4claude.yaml\n", string(content))
	})

	t.Run("appends to existing gitignore", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		gitignorePath := filepath.Join(dir, ".gitignore")
		require.NoError(t, os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0o644))

		result, err := yaml.Init(dir, "https://example.atlassian.net", "TEST")

		require.NoError(t, err)
		assert.True(t, result.GitignoreAdded)

		content, err := os.ReadFile(gitignorePath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "node_modules/")
		assert.Contains(t, string(content), ".jira4claude.yaml")
	})

	t.Run("skips gitignore if already contains entry", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		gitignorePath := filepath.Join(dir, ".gitignore")
		require.NoError(t, os.WriteFile(gitignorePath, []byte(".jira4claude.yaml\n"), 0o644))

		result, err := yaml.Init(dir, "https://example.atlassian.net", "TEST")

		require.NoError(t, err)
		assert.False(t, result.GitignoreAdded)
		assert.True(t, result.GitignoreExists)
	})

	t.Run("returns error when config file cannot be created", func(t *testing.T) {
		t.Parallel()

		// Use a non-existent directory as parent - writing will fail
		dir := filepath.Join(t.TempDir(), "nonexistent")

		_, err := yaml.Init(dir, "https://example.atlassian.net", "TEST")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "failed to create config file")
	})

	t.Run("returns error when gitignore cannot be read", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		gitignorePath := filepath.Join(dir, ".gitignore")
		// Create .gitignore as a directory - reading it as a file will fail
		require.NoError(t, os.Mkdir(gitignorePath, 0o755))

		_, err := yaml.Init(dir, "https://example.atlassian.net", "TEST")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "failed to read .gitignore")
	})

	t.Run("returns error when gitignore cannot be updated", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		gitignorePath := filepath.Join(dir, ".gitignore")
		// Create .gitignore as read-only - writing will fail
		require.NoError(t, os.WriteFile(gitignorePath, []byte("existing\n"), 0o444))

		_, err := yaml.Init(dir, "https://example.atlassian.net", "TEST")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "failed to update .gitignore")
	})

	t.Run("escapes special YAML characters in server and project", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		// Use values with special YAML characters: colons, quotes, newlines
		server := "https://example.atlassian.net: \"test\"\nnewline"
		project := "TEST: with \"quotes\" and\nnewline"

		result, err := yaml.Init(dir, server, project)

		require.NoError(t, err)
		assert.True(t, result.ConfigCreated)

		// The created file should be valid YAML that can be parsed back
		configPath := filepath.Join(dir, ".jira4claude.yaml")
		cfg, err := yaml.LoadConfig(configPath)
		require.NoError(t, err, "created config should be valid YAML")
		assert.Equal(t, server, cfg.Server, "server should round-trip correctly")
		assert.Equal(t, project, cfg.Project, "project should round-trip correctly")
	})
}

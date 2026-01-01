package yaml

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/fwojciec/jira4claude"
	"gopkg.in/yaml.v3"
)

const configFileName = ".jira4claude.yaml"

// Error constructors to reduce boilerplate.
func validationErr(msg string) error {
	return &jira4claude.Error{Code: jira4claude.EValidation, Message: msg}
}

func internalErr(msg string, inner error) error {
	return &jira4claude.Error{Code: jira4claude.EInternal, Message: msg, Inner: inner}
}

func notFoundErr(msg string, inner error) error {
	return &jira4claude.Error{Code: jira4claude.ENotFound, Message: msg, Inner: inner}
}

// configFile represents the YAML file structure.
// Field names are lowercase to match YAML keys.
type configFile struct {
	Server  string `yaml:"server"`
	Project string `yaml:"project"`
}

// LoadConfig loads configuration from a YAML file at the given path.
func LoadConfig(path string) (*jira4claude.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, notFoundErr("config file not found", err)
		}
		return nil, internalErr("failed to read config file", err)
	}

	var cf configFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EValidation,
			Message: "invalid YAML in config file",
			Inner:   err,
		}
	}

	if cf.Server == "" {
		return nil, validationErr("config file missing required field: server")
	}
	if cf.Project == "" {
		return nil, validationErr("config file missing required field: project")
	}

	return &jira4claude.Config{
		Server:  cf.Server,
		Project: cf.Project,
	}, nil
}

// DiscoverConfig searches for config files in standard locations.
// Returns the path to the first config file found.
// Search order: workDir/.jira4claude.yaml, homeDir/.jira4claude.yaml
func DiscoverConfig(workDir, homeDir string) (string, error) {
	localPath := filepath.Join(workDir, configFileName)
	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	globalPath := filepath.Join(homeDir, configFileName)
	if _, err := os.Stat(globalPath); err == nil {
		return globalPath, nil
	}

	return "", notFoundErr("no config file found; searched: ./"+configFileName+", ~/"+configFileName+"\nRun: j4c init --server=URL --project=KEY", nil)
}

// InitResult contains the result of the Init operation.
type InitResult struct {
	ConfigCreated   bool
	GitignoreAdded  bool
	GitignoreExists bool
}

// Init creates a new config file in the given directory.
func Init(dir, server, project string) (*InitResult, error) {
	configPath := filepath.Join(dir, configFileName)

	if err := validateConfigDoesNotExist(configPath); err != nil {
		return nil, err
	}

	if err := createConfigFile(configPath, server, project); err != nil {
		return nil, err
	}

	result := &InitResult{ConfigCreated: true}

	gitignorePath := filepath.Join(dir, ".gitignore")
	added, exists, err := ensureGitignoreEntry(gitignorePath, configFileName)
	if err != nil {
		return nil, err
	}
	result.GitignoreAdded = added
	result.GitignoreExists = exists

	return result, nil
}

// validateConfigDoesNotExist checks that no config file exists at the given path.
func validateConfigDoesNotExist(configPath string) error {
	if _, err := os.Stat(configPath); err == nil {
		return validationErr(configFileName + " already exists")
	}
	return nil
}

// createConfigFile writes a new config file with the given server and project.
func createConfigFile(configPath, server, project string) error {
	cf := configFile{
		Server:  server,
		Project: project,
	}
	content, err := yaml.Marshal(&cf)
	if err != nil {
		return internalErr("failed to marshal config", err)
	}
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		return internalErr("failed to create config file", err)
	}
	return nil
}

// ensureGitignoreEntry ensures the given entry is present in the gitignore file.
// Returns (added, alreadyExists, error).
func ensureGitignoreEntry(gitignorePath, entry string) (bool, bool, error) {
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, false, internalErr("failed to read .gitignore", err)
	}

	// Check if already in .gitignore (exact match on its own line)
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == entry {
			return false, true, nil
		}
	}

	// Append to .gitignore
	var newContent string
	if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
		newContent = string(content) + "\n" + entry + "\n"
	} else {
		newContent = string(content) + entry + "\n"
	}

	if err := os.WriteFile(gitignorePath, []byte(newContent), 0o644); err != nil {
		return false, false, internalErr("failed to update .gitignore", err)
	}

	return true, false, nil
}

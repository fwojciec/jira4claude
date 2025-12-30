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
			return nil, &jira4claude.Error{
				Code:    jira4claude.ENotFound,
				Message: "config file not found",
				Inner:   err,
			}
		}
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to read config file",
			Inner:   err,
		}
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
		return nil, &jira4claude.Error{
			Code:    jira4claude.EValidation,
			Message: "config file missing required field: server",
		}
	}
	if cf.Project == "" {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EValidation,
			Message: "config file missing required field: project",
		}
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

	return "", &jira4claude.Error{
		Code:    jira4claude.ENotFound,
		Message: "no config file found; searched: ./" + configFileName + ", ~/" + configFileName + "\nRun: jira4claude init --server=URL --project=KEY",
	}
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

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EValidation,
			Message: configFileName + " already exists",
		}
	}

	// Create config file
	content := "server: " + server + "\nproject: " + project + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create config file",
			Inner:   err,
		}
	}

	result := &InitResult{ConfigCreated: true}

	// Handle .gitignore
	gitignorePath := filepath.Join(dir, ".gitignore")
	gitignoreContent, err := os.ReadFile(gitignorePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to read .gitignore",
			Inner:   err,
		}
	}

	// Check if already in .gitignore
	if strings.Contains(string(gitignoreContent), configFileName) {
		result.GitignoreExists = true
		return result, nil
	}

	// Append to .gitignore
	var newContent string
	if len(gitignoreContent) > 0 && !strings.HasSuffix(string(gitignoreContent), "\n") {
		newContent = string(gitignoreContent) + "\n" + configFileName + "\n"
	} else {
		newContent = string(gitignoreContent) + configFileName + "\n"
	}

	if err := os.WriteFile(gitignorePath, []byte(newContent), 0o644); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to update .gitignore",
			Inner:   err,
		}
	}

	result.GitignoreAdded = true
	return result, nil
}

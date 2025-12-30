package yaml

import (
	"errors"
	"os"

	"github.com/fwojciec/jira4claude"
	"gopkg.in/yaml.v3"
)

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

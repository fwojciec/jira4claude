package main

import (
	"os"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/yaml"
)

// InitCmd initializes config.
type InitCmd struct {
	Server  string `help:"Jira server URL" required:""`
	Project string `help:"Default project key" required:""`
}

// Run executes the init command.
func (c *InitCmd) Run(ctx *MessageContext) error {
	workDir, err := os.Getwd()
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to get working directory",
			Inner:   err,
		}
	}

	result, err := yaml.Init(workDir, c.Server, c.Project)
	if err != nil {
		return err
	}

	if result.ConfigCreated {
		ctx.Printer.Success("Created .jira4claude.yaml")
	}
	if result.GitignoreAdded {
		ctx.Printer.Success("Added .jira4claude.yaml to .gitignore")
	} else if result.GitignoreExists {
		ctx.Printer.Success(".jira4claude.yaml already in .gitignore")
	}
	return nil
}

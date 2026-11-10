package main

import (
	"os"

	"github.com/fwojciec/jira4claude"
)

// InitCmd initializes config.
type InitCmd struct {
	Server  string `help:"Jira server URL" required:""`
	Project string `help:"Default project key" required:""`
}

// Run executes the init command.
func (c *InitCmd) Run(ctx *ConfigContext) error {
	workDir, err := os.Getwd()
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to get working directory",
			Inner:   err,
		}
	}

	result, err := ctx.Service.Init(workDir, c.Server, c.Project)
	if err != nil {
		return err
	}

	ctx.Printer.Initialized(*result)
	return nil
}

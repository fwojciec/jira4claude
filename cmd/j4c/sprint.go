package main

import (
	"context"

	"github.com/fwojciec/jira4claude"
)

// BoardCmd groups board subcommands.
type BoardCmd struct {
	List BoardListCmd `cmd:"" help:"List boards for a project"`
}

// SprintCmd groups sprint subcommands.
type SprintCmd struct {
	List SprintListCmd `cmd:"" help:"List sprints on a board"`
}

// SprintContext provides dependencies for board and sprint commands.
type SprintContext struct {
	BoardService  jira4claude.BoardService
	SprintService jira4claude.SprintService
	Printer       jira4claude.Printer
	Config        *jira4claude.Config
}

// BoardListCmd lists boards for a project.
type BoardListCmd struct {
	Project string `help:"Project key (defaults to config)" short:"p"`
}

// Run executes the board list command.
func (c *BoardListCmd) Run(ctx *SprintContext) error {
	project := c.Project
	if project == "" {
		project = ctx.Config.Project
	}

	boards, err := ctx.BoardService.ListBoards(context.Background(), project)
	if err != nil {
		return err
	}

	ctx.Printer.Boards(jira4claude.ToBoardViews(boards))
	return nil
}

// SprintListCmd lists sprints on a board.
type SprintListCmd struct {
	Board int    `arg:"" help:"Board ID"`
	State string `help:"Filter by state: active, future, closed (default: active,future)"`
}

// Run executes the sprint list command.
func (c *SprintListCmd) Run(ctx *SprintContext) error {
	var states []string
	if c.State != "" {
		states = []string{c.State}
	}

	sprints, err := ctx.SprintService.ListSprints(context.Background(), c.Board, states)
	if err != nil {
		return err
	}

	ctx.Printer.Sprints(jira4claude.ToSprintViews(sprints))
	return nil
}

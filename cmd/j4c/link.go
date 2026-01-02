package main

import (
	"context"

	"github.com/fwojciec/jira4claude"
)

// LinkCmd groups link subcommands.
type LinkCmd struct {
	Create LinkCreateCmd `cmd:"" help:"Create a link between issues"`
	Delete LinkDeleteCmd `cmd:"" help:"Delete a link between issues"`
	List   LinkListCmd   `cmd:"" help:"List links for an issue"`
}

// LinkCreateCmd creates a link.
type LinkCreateCmd struct {
	InwardKey  string `arg:"" help:"Source issue key"`
	LinkType   string `arg:"" help:"Link type (e.g., Blocks, Clones, Relates)"`
	OutwardKey string `arg:"" help:"Target issue key"`
}

// Run executes the create link command.
func (c *LinkCreateCmd) Run(ctx *LinkContext) error {
	if err := ctx.Service.Link(context.Background(), c.InwardKey, c.LinkType, c.OutwardKey); err != nil {
		return err
	}

	ctx.Printer.Success("Linked "+c.InwardKey+" "+c.LinkType, c.OutwardKey)
	return nil
}

// LinkDeleteCmd deletes a link.
type LinkDeleteCmd struct {
	Key1 string `arg:"" help:"First issue key"`
	Key2 string `arg:"" help:"Second issue key"`
}

// Run executes the delete link command.
func (c *LinkDeleteCmd) Run(ctx *LinkContext) error {
	if err := ctx.Service.Unlink(context.Background(), c.Key1, c.Key2); err != nil {
		return err
	}

	ctx.Printer.Success("Unlinked "+c.Key1+" and", c.Key2)
	return nil
}

// LinkListCmd lists links for an issue.
type LinkListCmd struct {
	Key string `arg:"" help:"Issue key"`
}

// Run executes the list links command.
func (c *LinkListCmd) Run(ctx *LinkContext) error {
	issue, err := ctx.Service.Get(context.Background(), c.Key)
	if err != nil {
		return err
	}

	links := jira4claude.ToLinksView(issue.Links)
	ctx.Printer.Links(c.Key, links)
	return nil
}

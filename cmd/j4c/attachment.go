package main

import (
	"context"

	"github.com/fwojciec/jira4claude"
)

// AttachmentCmd groups attachment subcommands.
type AttachmentCmd struct {
	List AttachmentListCmd `cmd:"" help:"List the attachments of an issue"`
}

// AttachmentContext provides dependencies for attachment commands.
type AttachmentContext struct {
	Service jira4claude.AttachmentService
	Printer jira4claude.Printer
	Config  *jira4claude.Config
}

// AttachmentListCmd lists the attachments of an issue.
type AttachmentListCmd struct {
	Key string `arg:"" help:"Issue key (e.g., PROJ-123)"`
}

// Run executes the attachment list command.
func (c *AttachmentListCmd) Run(ctx *AttachmentContext) error {
	attachments, err := ctx.Service.List(context.Background(), c.Key)
	if err != nil {
		return err
	}
	ctx.Printer.Attachments(c.Key, jira4claude.ToAttachmentViews(attachments))
	return nil
}

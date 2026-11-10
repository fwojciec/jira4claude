package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/fs"
)

// AttachmentCmd groups attachment subcommands.
type AttachmentCmd struct {
	List     AttachmentListCmd     `cmd:"" help:"List the attachments of an issue"`
	Download AttachmentDownloadCmd `cmd:"" help:"Save attachments of an issue into a directory; prints one saved path per line"`
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

// AttachmentDownloadCmd saves attachments of an issue to local files.
type AttachmentDownloadCmd struct {
	Key       string `arg:"" help:"Issue key (e.g., PROJ-123)"`
	ID        string `help:"Download the attachment with this ID" xor:"which"`
	Name      string `help:"Download the attachment with this exact filename (as Jira reports it, not as list escapes it)" xor:"which"`
	All       bool   `help:"Download every attachment of the issue" xor:"which"`
	Dir       string `help:"Existing directory to save into (not created)" required:""`
	Overwrite bool   `help:"Replace existing regular files (never symlinks or directories)"`
}

// Validate requires one of --id, --name and --all; kong's xor group makes
// them exclusive.
func (c *AttachmentDownloadCmd) Validate() error {
	if c.ID == "" && c.Name == "" && !c.All {
		return &jira4claude.Error{Code: jira4claude.EValidation, Message: "one of --id, --name or --all is required"}
	}
	return nil
}

// Run executes the attachment download command.
//
// The order keeps failures cheap and writes safe: the directory is checked
// before Jira is asked anything; the attachments are read from the issue, so
// an --id must belong to KEY; every local name is decided (and clashes
// resolved) before the first download; each file goes through fs.Dir.Save.
// A failure before the downloads (directory, list, names) stops the run.
//
// With several attachments, as issue view does with several keys, a failure
// to save one is printed as its own error and the others are still tried,
// except after an unauthorized, forbidden or rate-limit error, which would
// repeat for each (see stopsRun). The files saved are printed, and a summary
// error naming the failed IDs is returned with the code of the first
// failure. A single selected attachment that fails is returned as it is.
func (c *AttachmentDownloadCmd) Run(ctx *AttachmentContext) error {
	dir, err := fs.OpenDir(c.Dir, c.Overwrite)
	if err != nil {
		return err
	}
	defer func() {
		// Error ignored: closing a directory handle has no recovery path.
		_ = dir.Close()
	}()

	attachments, err := ctx.Service.List(context.Background(), c.Key)
	if err != nil {
		return err
	}
	selected, err := c.selectAttachments(attachments)
	if err != nil {
		return err
	}
	names, err := jira4claude.LocalNames(selected)
	if err != nil {
		return err
	}

	saved := make([]jira4claude.SavedAttachmentView, 0, len(selected))
	var failed, notTried []string
	var firstErr, stopErr error
	for i, a := range selected {
		if stopErr != nil {
			notTried = append(notTried, a.ID)
			continue
		}
		path, n, err := dir.Save(names[i], a.Size, func(w io.Writer) error {
			_, err := ctx.Service.Download(context.Background(), a.ID, w)
			return err
		})
		if err != nil {
			fileErr := &jira4claude.Error{
				Code:    jira4claude.ErrorCode(err),
				Message: fmt.Sprintf("attachment %s (%q)", a.ID, a.Filename),
				Inner:   err,
			}
			if len(selected) == 1 {
				return fileErr
			}
			ctx.Printer.Error(fileErr)
			failed = append(failed, a.ID)
			if firstErr == nil {
				firstErr = err
			}
			if stopsRun(err) {
				stopErr = err
			}
			continue
		}
		saved = append(saved, jira4claude.SavedAttachmentView{ID: a.ID, Filename: a.Filename, Path: path, Size: n})
	}

	// Printed even when every file failed: --json then prints [], and text
	// mode prints nothing rather than claiming the issue has no attachments.
	ctx.Printer.SavedAttachments(c.Key, saved, len(failed)+len(notTried))
	if len(failed) == 0 {
		return nil
	}
	msg := fmt.Sprintf("could not save %d of %d attachments: %s",
		len(failed)+len(notTried), len(selected), strings.Join(failed, ", "))
	if stopErr != nil {
		msg += fmt.Sprintf("; stopped after a %s error, not tried: %s",
			jira4claude.ErrorCode(stopErr), strings.Join(notTried, ", "))
	}
	return &jira4claude.Error{Code: jira4claude.ErrorCode(firstErr), Message: msg}
}

// stopsRun reports whether a failure to save one attachment would repeat for
// every other one, so that --all should stop: Jira refused the credentials,
// the permission or the request rate. Other failures (a file that exists, a
// broken connection, a size mismatch) are particular to one file.
func stopsRun(err error) bool {
	switch jira4claude.ErrorCode(err) {
	case jira4claude.EUnauthorized, jira4claude.EForbidden, jira4claude.ERateLimit:
		return true
	}
	return false
}

// selectAttachments picks the attachments named by --id, --name or --all.
func (c *AttachmentDownloadCmd) selectAttachments(attachments []*jira4claude.Attachment) ([]*jira4claude.Attachment, error) {
	switch {
	case c.All:
		return attachments, nil
	case c.ID != "":
		for _, a := range attachments {
			if a.ID == c.ID {
				return []*jira4claude.Attachment{a}, nil
			}
		}
		return nil, &jira4claude.Error{
			Code:    jira4claude.ENotFound,
			Message: fmt.Sprintf("attachment %s is not on %s", c.ID, c.Key),
		}
	default:
		var matches []*jira4claude.Attachment
		for _, a := range attachments {
			if a.Filename == c.Name {
				matches = append(matches, a)
			}
		}
		switch len(matches) {
		case 0:
			return nil, &jira4claude.Error{
				Code:    jira4claude.ENotFound,
				Message: fmt.Sprintf("no attachment of %s is named %q", c.Key, c.Name),
			}
		case 1:
			return matches, nil
		default:
			ids := make([]string, len(matches))
			for i, a := range matches {
				ids[i] = a.ID
			}
			return nil, &jira4claude.Error{
				Code: jira4claude.EValidation,
				Message: fmt.Sprintf("%d attachments of %s are named %q (%s); use --id",
					len(matches), c.Key, c.Name, strings.Join(ids, ", ")),
			}
		}
	}
}

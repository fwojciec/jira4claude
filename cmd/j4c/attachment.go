package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/fs"
)

// AttachmentCmd groups attachment subcommands.
type AttachmentCmd struct {
	List     AttachmentListCmd     `cmd:"" help:"List the attachments of an issue"`
	Download AttachmentDownloadCmd `cmd:"" help:"Save attachments of an issue into a directory; prints one saved path per line"`
	Upload   AttachmentUploadCmd   `cmd:"" help:"Attach local files to an issue, one request per file; prints one line per new attachment (ID, bytes, MIME type, filename) when the run ends. If a run is interrupted, attachment list KEY shows what was attached"`
	Delete   AttachmentDeleteCmd   `cmd:"" help:"Delete an attachment permanently, with no confirmation; prints the deleted attachment (ID, bytes, MIME type, filename). A delete Jira answers not found after the attachment was read counts as done: it was already gone"`
}

// AttachmentDeleteCmd deletes one attachment.
type AttachmentDeleteCmd struct {
	ID string `arg:"" help:"Attachment ID, as attachment list prints it"`
}

// Run executes the attachment delete command.
//
// The attachment is read first (one more request), so that the result names
// what was deleted, its filename and size, and a wrong ID fails before
// anything is deleted. Jira's attachment metadata does not name the issue,
// so the result has no key. There is no confirmation: the tool never
// prompts, and the delete cannot be undone.
//
// A delete answered not found after the read found the attachment is
// success: the attachment is gone, as asked, deleted by an earlier attempt of
// this delete that the server applied before failing (DELETE is retried on
// 5xx and connection errors) or by someone else meanwhile. The result is
// printed as usual, with a warning saying so. A not found on the read is an
// error.
func (c *AttachmentDeleteCmd) Run(ctx *AttachmentContext) error {
	a, err := ctx.Service.Get(context.Background(), c.ID)
	if err != nil {
		return err
	}
	if err := ctx.Service.Delete(context.Background(), c.ID); err != nil {
		if jira4claude.ErrorCode(err) != jira4claude.ENotFound {
			return err
		}
		ctx.Printer.Warning(fmt.Sprintf("attachment %s (%q) was read, but the delete answered not found: it was already gone, deleted by a retried attempt of this delete or by someone else meanwhile",
			c.ID, a.Filename))
	}
	ctx.Printer.Mutation(jira4claude.MutationResult{
		Action:      jira4claude.ActionAttachmentDeleted,
		Attachments: jira4claude.ToAttachmentViews([]*jira4claude.Attachment{a}),
	})
	return nil
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

// AttachmentUploadCmd attaches local files to an issue.
type AttachmentUploadCmd struct {
	Key   string   `arg:"" help:"Issue key (e.g., PROJ-123)"`
	Files []string `arg:"" name:"file" help:"Local files to attach, each sent under its base name; symlinks are followed"`
}

// Run executes the attachment upload command.
//
// Every file is checked and opened before Jira is asked anything (see
// fs.OpenUploads): a missing, unreadable or non-regular file, or a file given
// twice, fails the command with nothing sent. Two files with the same base
// name are both sent, with a warning: Jira keeps each as its own attachment.
//
// Each file is its own request, in the order given, so that a failure is
// reported for its file and a retry never sends again a file that was
// attached. As attachment download --all does, a failure is printed as its
// own error and the other files are still tried, except after an
// unauthorized, forbidden, not-found or rate-limit error, which would repeat
// for each (the issue is the same for all of them; see stopsUpload). The
// attachments created are printed as one result, and a summary error naming
// the failed files is returned with the code of the first failure; with none
// attached, --json still prints the result, with attachments []. A single
// file that fails is returned as it is, with nothing on stdout, as for any
// failed write.
//
// The result is printed when the run ends. If a run is interrupted, attachment
// list KEY shows what was attached.
func (c *AttachmentUploadCmd) Run(ctx *AttachmentContext) error {
	uploads, err := fs.OpenUploads(c.Files)
	if err != nil {
		return err
	}
	defer func() {
		// Error ignored: the files were only read.
		_ = uploads.Close()
	}()
	warnSameNames(ctx.Printer, uploads)

	added := make([]*jira4claude.Attachment, 0, len(uploads.Files))
	var failed, notTried []string
	var firstErr, stopErr error
	for i, file := range uploads.Files {
		path := uploads.Paths[i]
		if stopErr != nil {
			notTried = append(notTried, strconv.Quote(path))
			continue
		}
		a, err := ctx.Service.Upload(context.Background(), c.Key, file)
		if err != nil {
			if len(uploads.Files) == 1 {
				return err
			}
			ctx.Printer.Error(&jira4claude.Error{
				Code:    jira4claude.ErrorCode(err),
				Message: fmt.Sprintf("file %q", path),
				Inner:   err,
			})
			failed = append(failed, strconv.Quote(path))
			if firstErr == nil {
				firstErr = err
			}
			if stopsUpload(err) {
				stopErr = err
			}
			continue
		}
		added = append(added, a)
	}

	// Printed even when every file failed: --json then prints the result
	// with attachments [], so that stdout always parses; text mode prints a
	// line per attachment, so nothing.
	result := newResult(ctx.Config, c.Key, jira4claude.ActionAttachmentAdded)
	result.Attachments = jira4claude.ToAttachmentViews(added)
	ctx.Printer.Mutation(result)
	if len(failed) == 0 {
		return nil
	}
	msg := fmt.Sprintf("could not upload %d of %d files: %s",
		len(failed)+len(notTried), len(uploads.Files), strings.Join(failed, ", "))
	if stopErr != nil {
		msg += fmt.Sprintf("; stopped after a %s error, not tried: %s",
			jira4claude.ErrorCode(stopErr), strings.Join(notTried, ", "))
	}
	return &jira4claude.Error{Code: jira4claude.ErrorCode(firstErr), Message: msg}
}

// stopsUpload reports whether a failure to upload one file would repeat for
// every other one: those of stopsRun, and not found, which is about the issue
// (or the permission to see it) that every file goes to.
func stopsUpload(err error) bool {
	return stopsRun(err) || jira4claude.ErrorCode(err) == jira4claude.ENotFound
}

// warnSameNames warns about files sent under the same name: Jira accepts
// them, each as its own attachment, but a script matching by name cannot
// tell them apart.
func warnSameNames(printer jira4claude.MessagePrinter, uploads *fs.Uploads) {
	first := make(map[string]int, len(uploads.Files))
	for i, file := range uploads.Files {
		j, ok := first[file.Name]
		if !ok {
			first[file.Name] = i
			continue
		}
		printer.Warning(fmt.Sprintf("%q and %q are both sent as %q; Jira keeps each as its own attachment with its own ID",
			uploads.Paths[j], uploads.Paths[i], file.Name))
	}
}

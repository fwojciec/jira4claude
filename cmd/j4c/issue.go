package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fwojciec/jira4claude"
)

// IssueCmd groups issue subcommands.
type IssueCmd struct {
	View          IssueViewCmd          `cmd:"" help:"View one or more issues, whole or by field"`
	List          IssueListCmd          `cmd:"" help:"List issues"`
	Ready         IssueReadyCmd         `cmd:"" help:"List issues ready to work on"`
	Create        IssueCreateCmd        `cmd:"" help:"Create an issue"`
	Update        IssueUpdateCmd        `cmd:"" help:"Update an issue"`
	Delete        IssueDeleteCmd        `cmd:"" help:"Delete an issue"`
	Fields        IssueFieldsCmd        `cmd:"" help:"List settable fields for create or edit"`
	Transitions   IssueTransitionsCmd   `cmd:"" help:"List available transitions"`
	Transition    IssueTransitionCmd    `cmd:"" help:"Transition an issue"`
	Assign        IssueAssignCmd        `cmd:"" help:"Assign an issue"`
	Comment       IssueCommentCmd       `cmd:"" help:"Add a comment to an issue"`
	DeleteComment IssueDeleteCommentCmd `cmd:"delete-comment" help:"Delete a comment from an issue"`
}

// IssueViewCmd views one or more issues, whole or by field.
type IssueViewCmd struct {
	Keys  []string `arg:"" name:"key" help:"Issue keys (e.g., PROJ-123); several are printed in the order given"`
	Field []string `name:"field" sep:"none" placeholder:"NAME" help:"Print only this field (repeatable or comma-separated): key, project, summary, description, status, type, priority, assignee, reporter, labels, relatedIssues, comments, attachments, created, updated, url, or a custom field ID (customfield_NNNNN). One key and one field print the raw value; otherwise each value follows a line '==> KEY FIELD <=='. With --json, one key gives an object of key and fields, several keys an array of them"`
}

// Run executes the view command. One key behaves as it always has: a failed
// read is the command's error. With several keys every key is read; the
// issues read are printed in the order asked, each failure is reported on
// stderr as "KEY: message", and the command fails with the code of the first
// failure in the order asked.
func (c *IssueViewCmd) Run(ctx *IssueContext) error {
	// Split commas here rather than in the parser, which drops an empty
	// value: --field "$UNSET" must fail, not print the whole issue.
	names := make([]string, 0, len(c.Field))
	for _, f := range c.Field {
		names = append(names, strings.Split(f, ",")...)
	}
	fields, err := jira4claude.ParseViewFields(names)
	if err != nil {
		return err
	}

	several := len(c.Keys) > 1
	views := make([]jira4claude.IssueView, 0, len(c.Keys))
	selection := jira4claude.IssueSelectionView{Fields: fields, Several: several}
	var failed []string
	var firstErr error
	for _, key := range c.Keys {
		issue, err := ctx.Service.Get(context.Background(), key)
		if err != nil {
			if len(c.Keys) == 1 {
				return err
			}
			ctx.Printer.Error(&jira4claude.Error{Code: jira4claude.ErrorCode(err), Message: key, Inner: err})
			failed = append(failed, key)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		// Warnings name the issue by the key Jira returned, as the output does.
		warn := func(ci jira4claude.ConversionIssue) {
			ctx.Printer.ViewConversionIssue(jira4claude.ViewConversionIssue{ConversionIssue: ci, Key: issue.Key, Several: several})
		}
		if len(fields) > 0 {
			selection.Issues = append(selection.Issues,
				jira4claude.ToSelectedIssueView(issue, fields, ctx.Converter, warn, ctx.Config.Server))
			continue
		}
		views = append(views, jira4claude.ToIssueView(issue, ctx.Converter, warn, ctx.Config.Server))
	}

	switch {
	case len(fields) > 0:
		ctx.Printer.IssueSelection(selection)
	case several:
		ctx.Printer.IssueViews(views)
	default:
		ctx.Printer.Issue(views[0])
	}

	if len(failed) > 0 {
		return &jira4claude.Error{
			Code: jira4claude.ErrorCode(firstErr),
			Message: fmt.Sprintf("could not read %d of %d issues: %s",
				len(failed), len(c.Keys), strings.Join(failed, ", ")),
		}
	}
	return nil
}

// IssueListCmd lists issues.
type IssueListCmd struct {
	Project       string   `help:"Filter by project" short:"p"`
	Status        string   `help:"Filter by status" short:"s"`
	ExcludeStatus string   `help:"Exclude issues with this status" name:"exclude-status"`
	Assignee      string   `help:"Filter by assignee (use 'me' for current user)" short:"a"`
	Parent        string   `help:"Filter by parent issue" short:"P"`
	Labels        []string `help:"Filter by labels" short:"l"`
	OrderBy       string   `help:"Order results (e.g., 'created DESC')" name:"order-by"`
	JQL           string   `help:"Raw JQL query (overrides other filters)"`
	Limit         int      `help:"Maximum number of results" default:"50"`
}

// Run executes the list command.
func (c *IssueListCmd) Run(ctx *IssueContext) error {
	filter := jira4claude.IssueFilter{
		Project:       c.Project,
		Status:        c.Status,
		ExcludeStatus: c.ExcludeStatus,
		Assignee:      c.Assignee,
		Parent:        c.Parent,
		Labels:        c.Labels,
		OrderBy:       c.OrderBy,
		JQL:           c.JQL,
		Limit:         c.Limit,
	}
	if filter.Project == "" && filter.JQL == "" {
		filter.Project = ctx.Config.Project
	}

	issues, next, err := ctx.Service.List(context.Background(), filter)
	if err != nil {
		return err
	}
	truncated := next != ""
	if truncated {
		msg := fmt.Sprintf("showing the first %d issues; more match", len(issues))
		// Jira may return a short page before --limit is reached; raising
		// --limit only helps when the page was cut at the limit.
		if c.Limit > 0 && len(issues) >= c.Limit {
			msg += " (raise --limit to see them)"
		}
		ctx.Printer.Warning(msg)
	}
	ctx.Printer.Issues(jira4claude.IssueListView{
		Issues:    jira4claude.ToIssueListItems(issues),
		Truncated: truncated,
	})
	return nil
}

// IssueReadyCmd lists ready issues.
type IssueReadyCmd struct {
	Project string `help:"Filter by project" short:"p"`
	Parent  string `help:"Filter by parent issue" short:"P"`
	Limit   int    `help:"Maximum number of results" default:"50"`
}

// Run executes the ready command.
func (c *IssueReadyCmd) Run(ctx *IssueContext) error {
	project := c.Project
	if project == "" {
		project = ctx.Config.Project
	}

	filter := jira4claude.IssueFilter{
		Project:       project,
		Parent:        c.Parent,
		ExcludeStatus: "Done",
		OrderBy:       "created DESC",
		Limit:         c.Limit,
	}

	// Readiness is decided client-side, so page through the search until we
	// have Limit ready issues or the results end. Limit <= 0 means no cap.
	var ready []*jira4claude.Issue
	truncated := false
	for {
		issues, next, err := ctx.Service.List(context.Background(), filter)
		if err != nil {
			return err
		}
		for _, issue := range issues {
			if jira4claude.IsReady(issue) {
				ready = append(ready, issue)
			}
		}
		if c.Limit > 0 && len(ready) >= c.Limit {
			truncated = len(ready) > c.Limit || next != ""
			ready = ready[:c.Limit]
			break
		}
		if next == "" {
			break
		}
		filter.PageToken = next
	}

	if truncated {
		ctx.Printer.Warning(fmt.Sprintf("showing the first %d ready issues; more may exist (raise --limit to see them)", len(ready)))
	}
	ctx.Printer.Issues(jira4claude.IssueListView{
		Issues:    jira4claude.ToIssueListItems(ready),
		Truncated: truncated,
	})
	return nil
}

// IssueCreateCmd creates an issue.
type IssueCreateCmd struct {
	Project     string   `help:"Project key" short:"p"`
	Type        string   `help:"Issue type" short:"t" default:"Task"`
	Summary     string   `help:"Issue summary" short:"s" required:""`
	Description string   `help:"Issue description" short:"d"`
	Priority    string   `help:"Issue priority"`
	Labels      []string `help:"Issue labels" short:"l"`
	Parent      string   `help:"Parent issue key (pass --type=Sub-task explicitly for classic-project sub-tasks)" short:"P"`
	Assignee    string   `help:"Assignee: 'me', email, or account ID" short:"A"`
	FieldJSON   []string `name:"field-json" sep:"none" help:"Set field by ID, value is JSON (repeatable). Example: customfield_10801='{\"value\":\"High\"}'"`
	Force       bool     `help:"Write even if Markdown conversion drops content (e.g. images); dropped content is reported as a warning"`
}

// Run executes the create command.
func (c *IssueCreateCmd) Run(ctx *IssueContext) error {
	project := c.Project
	if project == "" {
		project = ctx.Config.Project
	}

	// Convert description to ADF (plain text is valid GFM)
	var description *jira4claude.ADFNode
	if c.Description != "" {
		var err error
		description, err = convertForWrite(ctx, "description", c.Description, c.Force)
		if err != nil {
			return err
		}
	}

	var parent *jira4claude.LinkedIssue
	if c.Parent != "" {
		parent = &jira4claude.LinkedIssue{Key: c.Parent}
	}

	customFields, err := ParseFieldJSON(c.FieldJSON)
	if err != nil {
		return err
	}

	// Resolve the assignee before any write, so a bad value fails without
	// leaving an issue behind.
	var assignee *jira4claude.User
	if c.Assignee != "" {
		accountID, err := ResolveAssignee(context.Background(), c.Assignee, ctx.UserService)
		if err != nil {
			return err
		}
		assignee = &jira4claude.User{AccountID: accountID}
	}

	issue := &jira4claude.Issue{
		Project:      project,
		Type:         c.Type,
		Summary:      c.Summary,
		Description:  description,
		Priority:     c.Priority,
		Labels:       c.Labels,
		Parent:       parent,
		Assignee:     assignee,
		CustomFields: customFields,
	}

	created, err := ctx.Service.Create(context.Background(), issue)
	if err == nil {
		ctx.Printer.Success("Created:", created.Key)
		return nil
	}
	if assignee == nil || !rejectsAssigneeField(err) {
		return err
	}

	// The project's create screen does not accept the assignee. Jira refused
	// the whole request, so nothing was created and a second create is safe.
	ctx.Printer.Warning("Jira rejected the assignee on create (" + jira4claude.ErrorMessage(err) +
		"); creating without it and assigning separately")
	withoutAssignee := *issue
	withoutAssignee.Assignee = nil
	created, err = ctx.Service.Create(context.Background(), &withoutAssignee)
	if err != nil {
		return err
	}
	ctx.Printer.Success("Created:", created.Key)

	if err := ctx.Service.Assign(context.Background(), created.Key, assignee.AccountID); err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.ErrorCode(err),
			Message: "created " + created.Key + ", assignment failed",
			Inner:   err,
		}
	}
	return nil
}

// rejectsAssigneeField reports whether err is Jira's 400 answer to a create
// whose field errors name the assignee. Only a definite 400 maps to
// EValidation: 5xx and connection failures on POST, which may have been
// applied, carry other codes. ParseErrorResponse flattens field errors into
// "field: message" segments joined by "; ", so the assignee is named when a
// segment starts with "assignee: ".
func rejectsAssigneeField(err error) bool {
	var e *jira4claude.Error
	if !errors.As(err, &e) || e.Code != jira4claude.EValidation {
		return false
	}
	for _, segment := range strings.Split(e.Message, "; ") {
		if strings.HasPrefix(segment, "assignee: ") {
			return true
		}
	}
	return false
}

// IssueUpdateCmd updates an issue.
type IssueUpdateCmd struct {
	Key               string   `arg:"" help:"Issue key"`
	Summary           *string  `help:"New summary" short:"s"`
	Description       *string  `help:"Replace the description. Edit the Markdown issue view prints: each {{...}} token in it stands for an existing image, mention or other node that Markdown cannot carry, and is put back unchanged; leaving a token out is refused unless --force" short:"d" xor:"description"`
	AppendDescription *string  `help:"Add text after the current description; the existing content is kept as it is, never converted" xor:"description"`
	ClearDescription  bool     `help:"Remove the description; refused unless --force when it holds images, mentions or other nodes Markdown cannot carry" xor:"description"`
	Priority          *string  `help:"New priority"`
	Assignee          *string  `help:"Assignee: 'me', email, or account ID" short:"a"`
	Labels            []string `help:"New labels" short:"l"`
	ClearLabels       bool     `help:"Clear all labels" name:"clear-labels"`
	Parent            *string  `help:"Parent issue key" short:"P" xor:"parent"`
	ClearParent       bool     `help:"Remove from parent" name:"clear-parent" xor:"parent"`
	FieldJSON         []string `name:"field-json" sep:"none" help:"Set field by ID, value is JSON (repeatable). Example: customfield_10801='{\"value\":\"High\"}'"`
	IfUpdated         *string  `name:"if-updated" help:"Write only if the issue was last updated at this time, as issue view --json prints it in 'updated' (compared to the millisecond); otherwise exit 5 and write nothing. The check is made just before the write, so a change landing in between is not caught"`
	Force             bool     `help:"Write even if content would be lost: new Markdown that does not convert (e.g. images), existing nodes whose tokens were left out, or a cleared description that holds them; what is lost is reported as a warning"`
}

// Run executes the update command. Anything that touches the description, and
// --if-updated, reads the issue first: the read is the base for re-attaching
// protected nodes, appending and the --if-updated check, and comes as late as
// possible to keep the window before the write short.
func (c *IssueUpdateCmd) Run(ctx *IssueContext) error {
	// Validate and convert the new Markdown first, so a refusal that does not
	// depend on the current issue happens before any service call, including
	// assignee lookup.
	var ifUpdated *time.Time
	if c.IfUpdated != nil {
		t, err := jira4claude.ParseTimestamp(*c.IfUpdated)
		if err != nil {
			return err
		}
		ifUpdated = &t
	}
	var edited, addition *jira4claude.ADFNode
	var warnings []jira4claude.ConversionIssue
	switch {
	case c.Description != nil:
		if *c.Description == "" {
			return &jira4claude.Error{
				Code:    jira4claude.EValidation,
				Message: "--description cannot be empty; use --clear-description to remove the description",
			}
		}
		// Plain text is valid GFM.
		doc, issues, err := convert(ctx, "description", *c.Description, c.Force)
		if err != nil {
			return err
		}
		edited, warnings = doc, issues
	case c.AppendDescription != nil:
		if *c.AppendDescription == "" {
			return &jira4claude.Error{
				Code:    jira4claude.EValidation,
				Message: "--append-description cannot be empty",
			}
		}
		doc, issues, err := convert(ctx, "description", *c.AppendDescription, c.Force)
		if err != nil {
			return err
		}
		addition, warnings = doc, append(issues, jira4claude.LiteralTokens(ctx.Converter, doc)...)
	}

	// Resolve assignee if provided
	if c.Assignee != nil {
		accountID, err := ResolveAssignee(context.Background(), *c.Assignee, ctx.UserService)
		if err != nil {
			return err
		}
		c.Assignee = &accountID
	}

	customFields, err := ParseFieldJSON(c.FieldJSON)
	if err != nil {
		return err
	}

	var description jira4claude.DescriptionUpdate
	if edited != nil || addition != nil || c.ClearDescription || ifUpdated != nil {
		current, err := ctx.Service.Get(context.Background(), c.Key)
		if err != nil {
			return err
		}
		if ifUpdated != nil {
			if err := jira4claude.CheckUpdated(current, *ifUpdated); err != nil {
				return err
			}
		}
		switch {
		case edited != nil:
			doc, issues, err := jira4claude.ReplaceDescription(ctx.Converter, current.Description, edited, c.Force)
			if err != nil {
				return err
			}
			description = jira4claude.SetDescription(doc)
			warnings = append(warnings, issues...)
		case addition != nil:
			// Append at the ADF level: the current nodes are sent back as
			// read, never through Markdown.
			description = jira4claude.AppendDescription(current.Description, addition)
		case c.ClearDescription:
			issues, err := jira4claude.CheckClearDescription(ctx.Converter, current.Description, c.Force)
			if err != nil {
				return err
			}
			description = jira4claude.ClearDescription()
			warnings = append(warnings, issues...)
		}
	}
	for _, w := range warnings {
		ctx.Printer.ConversionIssue(w)
	}

	update := jira4claude.IssueUpdate{
		Summary:      c.Summary,
		Description:  description,
		Priority:     c.Priority,
		Assignee:     c.Assignee,
		CustomFields: customFields,
	}

	if len(c.Labels) > 0 {
		update.Labels = &c.Labels
	} else if c.ClearLabels {
		empty := []string{}
		update.Labels = &empty
	}

	if c.Parent != nil {
		update.Parent = c.Parent
	} else if c.ClearParent {
		empty := ""
		update.Parent = &empty
	}

	updated, err := ctx.Service.Update(context.Background(), c.Key, update)
	if err != nil {
		return err
	}

	ctx.Printer.Success("Updated:", updated.Key)
	return nil
}

// IssueFieldsCmd lists fields settable on create or edit for an issue type or
// specific issue.
//
// Modes are mutually exclusive:
//   - --key=KEY → edit discovery via GetEditFields.
//   - --project / --type (or defaults) → create discovery via GetCreateFields.
//
// The Kong xor pairings reject --key with either --project or --type, while
// permitting --project + --type together. Type carries no Kong-level default
// (Kong v1.13.0 treats defaulted flags as user-set for xor purposes, which
// would force every bare --key invocation into a conflict). The "Task"
// fallback is applied in Run() when Type is empty.
type IssueFieldsCmd struct {
	Project string `help:"Project key (for create-field discovery)" short:"p" xor:"mode-project"`
	Type    string `help:"Issue type (for create-field discovery; defaults to Task)" short:"t" xor:"mode-type"`
	Key     string `help:"Issue key (for edit-field discovery)" short:"k" xor:"mode-project,mode-type"`
	All     bool   `help:"Include all fields (default: required + custom only)"`
	Filter  string `help:"Show only fields whose name or ID contains this substring (case-insensitive, searches all fields)" short:"f"`
}

// Run executes the fields command.
func (c *IssueFieldsCmd) Run(ctx *IssueContext) error {
	var fields []*jira4claude.IssueField
	var source string

	if c.Key != "" {
		var err error
		fields, err = ctx.Service.GetEditFields(context.Background(), c.Key)
		if err != nil {
			return err
		}
		source = c.Key + " (edit)"
	} else {
		project := c.Project
		if project == "" {
			project = ctx.Config.Project
		}
		if project == "" {
			return &jira4claude.Error{
				Code:    jira4claude.EValidation,
				Message: "project required (--project or config)",
			}
		}
		issueType := c.Type
		if issueType == "" {
			issueType = "Task"
		}
		var err error
		fields, err = ctx.Service.GetCreateFields(context.Background(), project, issueType)
		if err != nil {
			return err
		}
		source = project + " / " + issueType
	}

	selected, scope, omitted := jira4claude.SelectIssueFields(fields, c.Filter, c.All)

	ctx.Printer.Fields(jira4claude.IssueFieldsView{
		Source:  source,
		Scope:   scope,
		Omitted: omitted,
		Fields:  selected,
	})
	return nil
}

// IssueTransitionsCmd lists available transitions.
type IssueTransitionsCmd struct {
	Key string `arg:"" help:"Issue key"`
}

// Run executes the transitions command.
func (c *IssueTransitionsCmd) Run(ctx *IssueContext) error {
	transitions, err := ctx.Service.Transitions(context.Background(), c.Key)
	if err != nil {
		return err
	}
	ctx.Printer.Transitions(c.Key, transitions)
	return nil
}

// IssueTransitionCmd transitions an issue.
type IssueTransitionCmd struct {
	Key    string `arg:"" help:"Issue key"`
	Status string `help:"Target status name" short:"s" xor:"target"`
	ID     string `help:"Transition ID" short:"i" xor:"target"`
}

// Run executes the transition command.
func (c *IssueTransitionCmd) Run(ctx *IssueContext) error {
	if c.Status == "" && c.ID == "" {
		return &jira4claude.Error{
			Code:    jira4claude.EValidation,
			Message: "either --status or --id is required",
		}
	}

	transitions, err := ctx.Service.Transitions(context.Background(), c.Key)
	if err != nil {
		return err
	}

	var transitionID string
	if c.ID != "" {
		transitionID = c.ID
	} else {
		for _, t := range transitions {
			if strings.EqualFold(t.Name, c.Status) {
				transitionID = t.ID
				break
			}
		}
		if transitionID == "" {
			available := make([]string, len(transitions))
			for i, t := range transitions {
				available[i] = `"` + t.Name + `"`
			}
			return &jira4claude.Error{
				Code:    jira4claude.EValidation,
				Message: `status "` + c.Status + `" not found; available: ` + strings.Join(available, ", "),
			}
		}
	}

	if err := ctx.Service.Transition(context.Background(), c.Key, transitionID); err != nil {
		return err
	}

	ctx.Printer.Success("Transitioned:", c.Key)
	return nil
}

// IssueAssignCmd assigns an issue.
type IssueAssignCmd struct {
	Key      string `arg:"" help:"Issue key"`
	Assignee string `help:"Assignee: 'me', email, or account ID (omit to unassign)" short:"a"`
}

// Run executes the assign command.
func (c *IssueAssignCmd) Run(ctx *IssueContext) error {
	accountID, err := ResolveAssignee(context.Background(), c.Assignee, ctx.UserService)
	if err != nil {
		return err
	}

	if err := ctx.Service.Assign(context.Background(), c.Key, accountID); err != nil {
		return err
	}

	if accountID == "" {
		ctx.Printer.Success("Unassigned:", c.Key)
	} else {
		ctx.Printer.Success("Assigned:", c.Key)
	}
	return nil
}

// IssueCommentCmd adds a comment.
type IssueCommentCmd struct {
	Key   string `arg:"" help:"Issue key"`
	Body  string `help:"Comment body" short:"b" required:""`
	Force bool   `help:"Write even if Markdown conversion drops content (e.g. images); dropped content is reported as a warning"`
}

// Run executes the comment command.
func (c *IssueCommentCmd) Run(ctx *IssueContext) error {
	// Convert body to ADF (plain text is valid GFM)
	body, err := convertForWrite(ctx, "comment body", c.Body, c.Force)
	if err != nil {
		return err
	}

	comment, err := ctx.Service.AddComment(context.Background(), c.Key, body)
	if err != nil {
		return err
	}

	ctx.Printer.Success("Added comment "+comment.ID+" to", c.Key)
	return nil
}

// IssueDeleteCmd deletes an issue.
type IssueDeleteCmd struct {
	Key            string `arg:"" help:"Issue key (e.g., PROJ-123)"`
	DeleteSubtasks bool   `name:"delete-subtasks" help:"Also delete sub-tasks (required by Jira when the issue has any)"`
}

// Run executes the delete command.
func (c *IssueDeleteCmd) Run(ctx *IssueContext) error {
	if err := ctx.Service.Delete(context.Background(), c.Key, c.DeleteSubtasks); err != nil {
		return err
	}
	// Key goes in the message rather than as a variadic key arg, so printers
	// don't render a /browse/<key> URL for an issue that no longer exists.
	ctx.Printer.Success("Deleted " + c.Key)
	return nil
}

// IssueDeleteCommentCmd deletes a comment from an issue.
type IssueDeleteCommentCmd struct {
	Key       string `arg:"" help:"Issue key"`
	CommentID string `arg:"" name:"comment-id" help:"Comment ID (visible in 'issue view' output)"`
}

// Run executes the delete-comment command.
func (c *IssueDeleteCommentCmd) Run(ctx *IssueContext) error {
	if err := ctx.Service.DeleteComment(context.Background(), c.Key, c.CommentID); err != nil {
		return err
	}
	ctx.Printer.Success("Deleted comment "+c.CommentID+" from", c.Key)
	return nil
}

// convertForWrite converts markdown to ADF for a write. Every conversion issue
// is reported as a warning. If any content was dropped and force is false, it
// returns a validation error naming the dropped nodes instead, so the caller
// writes nothing. field names the input in that error (e.g. "description").
// It is for new content (a create or a comment), so text that reads as a
// protected-node token is also reported: nothing restores it there.
func convertForWrite(ctx *IssueContext, field, md string, force bool) (*jira4claude.ADFNode, error) {
	doc, issues, err := convert(ctx, field, md, force)
	if err != nil {
		return nil, err
	}
	issues = append(issues, jira4claude.LiteralTokens(ctx.Converter, doc)...)
	for _, issue := range issues {
		ctx.Printer.ConversionIssue(issue)
	}
	return doc, nil
}

// convert is convertForWrite without printing the issues, for a caller that
// may still refuse the write and so reports them later.
func convert(ctx *IssueContext, field, md string, force bool) (*jira4claude.ADFNode, []jira4claude.ConversionIssue, error) {
	doc, issues := ctx.Converter.ToADF(md)
	if !force {
		if err := jira4claude.DroppedContentError(field, issues); err != nil {
			return nil, nil, err
		}
	}
	return doc, issues, nil
}

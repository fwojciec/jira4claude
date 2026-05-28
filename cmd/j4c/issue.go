package main

import (
	"context"
	"strings"

	"github.com/fwojciec/jira4claude"
)

// IssueCmd groups issue subcommands.
type IssueCmd struct {
	View          IssueViewCmd          `cmd:"" help:"View an issue"`
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

// IssueViewCmd views an issue.
type IssueViewCmd struct {
	Key string `arg:"" help:"Issue key (e.g., PROJ-123)"`
}

// Run executes the view command.
func (c *IssueViewCmd) Run(ctx *IssueContext) error {
	issue, err := ctx.Service.Get(context.Background(), c.Key)
	if err != nil {
		return err
	}
	view := jira4claude.ToIssueView(issue, ctx.Converter, ctx.Printer.Warning, ctx.Config.Server)
	ctx.Printer.Issue(view)
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

	issues, err := ctx.Service.List(context.Background(), filter)
	if err != nil {
		return err
	}
	items := jira4claude.ToIssueListItems(issues)
	ctx.Printer.Issues(items)
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

	issues, err := ctx.Service.List(context.Background(), filter)
	if err != nil {
		return err
	}

	ready := make([]*jira4claude.Issue, 0, len(issues))
	for _, issue := range issues {
		if jira4claude.IsReady(issue) {
			ready = append(ready, issue)
		}
	}

	items := jira4claude.ToIssueListItems(ready)
	ctx.Printer.Issues(items)
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
	FieldJSON   []string `name:"field-json" help:"Set field by ID, value is JSON (repeatable). Example: customfield_10801='{\"value\":\"High\"}'"`
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
		var warnings []string
		description, warnings = ctx.Converter.ToADF(c.Description)
		for _, w := range warnings {
			ctx.Printer.Warning(w)
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

	issue := &jira4claude.Issue{
		Project:      project,
		Type:         c.Type,
		Summary:      c.Summary,
		Description:  description,
		Priority:     c.Priority,
		Labels:       c.Labels,
		Parent:       parent,
		CustomFields: customFields,
	}

	created, err := ctx.Service.Create(context.Background(), issue)
	if err != nil {
		return err
	}

	ctx.Printer.Success("Created:", created.Key)

	if c.Assignee != "" {
		accountID, err := ResolveAssignee(context.Background(), c.Assignee, ctx.UserService)
		if err != nil {
			return err
		}
		if err := ctx.Service.Assign(context.Background(), created.Key, accountID); err != nil {
			return err
		}
	}

	return nil
}

// IssueUpdateCmd updates an issue.
type IssueUpdateCmd struct {
	Key         string   `arg:"" help:"Issue key"`
	Summary     *string  `help:"New summary" short:"s"`
	Description *string  `help:"New description" short:"d"`
	Priority    *string  `help:"New priority"`
	Assignee    *string  `help:"Assignee: 'me', email, or account ID" short:"a"`
	Labels      []string `help:"New labels" short:"l"`
	ClearLabels bool     `help:"Clear all labels" name:"clear-labels"`
	Parent      *string  `help:"Parent issue key" short:"P" xor:"parent"`
	ClearParent bool     `help:"Remove from parent" name:"clear-parent" xor:"parent"`
	FieldJSON   []string `name:"field-json" help:"Set field by ID, value is JSON (repeatable). Example: customfield_10801='{\"value\":\"High\"}'"`
}

// Run executes the update command.
func (c *IssueUpdateCmd) Run(ctx *IssueContext) error {
	// Resolve assignee if provided
	if c.Assignee != nil {
		accountID, err := ResolveAssignee(context.Background(), *c.Assignee, ctx.UserService)
		if err != nil {
			return err
		}
		c.Assignee = &accountID
	}

	// Convert description to ADF (plain text is valid GFM)
	var description **jira4claude.ADFNode
	if c.Description != nil && *c.Description != "" {
		adfDoc, warnings := ctx.Converter.ToADF(*c.Description)
		for _, w := range warnings {
			ctx.Printer.Warning(w)
		}
		description = &adfDoc
	}

	customFields, err := ParseFieldJSON(c.FieldJSON)
	if err != nil {
		return err
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

	filtered := fields
	if !c.All {
		filtered = make([]*jira4claude.IssueField, 0, len(fields))
		for _, f := range fields {
			if f.Required || strings.HasPrefix(f.ID, "customfield_") {
				filtered = append(filtered, f)
			}
		}
	}

	ctx.Printer.Fields(jira4claude.IssueFieldsView{
		Source: source,
		Fields: filtered,
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
	Key  string `arg:"" help:"Issue key"`
	Body string `help:"Comment body" short:"b" required:""`
}

// Run executes the comment command.
func (c *IssueCommentCmd) Run(ctx *IssueContext) error {
	// Convert body to ADF (plain text is valid GFM)
	body, warnings := ctx.Converter.ToADF(c.Body)
	for _, w := range warnings {
		ctx.Printer.Warning(w)
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
	Key string `arg:"" help:"Issue key (e.g., PROJ-123)"`
}

// Run executes the delete command.
func (c *IssueDeleteCmd) Run(ctx *IssueContext) error {
	if err := ctx.Service.Delete(context.Background(), c.Key); err != nil {
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

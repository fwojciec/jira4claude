package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/adf"
)

// markdownToADFJSON converts markdown text to ADF JSON string.
// The returned string can be passed to the issue service; the HTTP layer
// will detect it as pre-converted ADF and use it directly.
func markdownToADFJSON(markdown string) (string, error) {
	converter := adf.New()
	adfDoc, _ := converter.ToADF(markdown)
	bytes, err := json.Marshal(adfDoc)
	if err != nil {
		return "", &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to serialize ADF",
			Inner:   err,
		}
	}
	return string(bytes), nil
}

// IssueCmd groups issue subcommands.
type IssueCmd struct {
	View        IssueViewCmd        `cmd:"" help:"View an issue"`
	List        IssueListCmd        `cmd:"" help:"List issues"`
	Ready       IssueReadyCmd       `cmd:"" help:"List issues ready to work on"`
	Create      IssueCreateCmd      `cmd:"" help:"Create an issue"`
	Update      IssueUpdateCmd      `cmd:"" help:"Update an issue"`
	Transitions IssueTransitionsCmd `cmd:"" help:"List available transitions"`
	Transition  IssueTransitionCmd  `cmd:"" help:"Transition an issue"`
	Assign      IssueAssignCmd      `cmd:"" help:"Assign an issue"`
	Comment     IssueCommentCmd     `cmd:"" help:"Add a comment to an issue"`
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

	// Always convert ADF to markdown when available.
	// This ensures consistent output whether data comes from HTTP layer or mocks.
	converter := adf.New()
	if issue.DescriptionADF != nil {
		if desc, _ := converter.ToMarkdown(issue.DescriptionADF); desc != "" {
			issue.Description = desc
		}
	}
	for _, comment := range issue.Comments {
		if comment.BodyADF != nil {
			if body, _ := converter.ToMarkdown(comment.BodyADF); body != "" {
				comment.Body = body
			}
		}
	}

	ctx.Printer.Issue(issue)
	return nil
}

// IssueListCmd lists issues.
type IssueListCmd struct {
	Project  string   `help:"Filter by project" short:"p"`
	Status   string   `help:"Filter by status" short:"s"`
	Assignee string   `help:"Filter by assignee (use 'me' for current user)" short:"a"`
	Parent   string   `help:"Filter by parent issue" short:"P"`
	Labels   []string `help:"Filter by labels" short:"l"`
	JQL      string   `help:"Raw JQL query (overrides other filters)"`
	Limit    int      `help:"Maximum number of results" default:"50"`
}

// Run executes the list command.
func (c *IssueListCmd) Run(ctx *IssueContext) error {
	filter := jira4claude.IssueFilter{
		Project:  c.Project,
		Status:   c.Status,
		Assignee: c.Assignee,
		Parent:   c.Parent,
		Labels:   c.Labels,
		JQL:      c.JQL,
		Limit:    c.Limit,
	}
	if filter.Project == "" && filter.JQL == "" {
		filter.Project = ctx.Config.Project
	}

	issues, err := ctx.Service.List(context.Background(), filter)
	if err != nil {
		return err
	}
	ctx.Printer.Issues(issues)
	return nil
}

// IssueReadyCmd lists ready issues.
type IssueReadyCmd struct {
	Project string `help:"Filter by project" short:"p"`
	Limit   int    `help:"Maximum number of results" default:"50"`
}

// Run executes the ready command.
func (c *IssueReadyCmd) Run(ctx *IssueContext) error {
	project := c.Project
	if project == "" {
		project = ctx.Config.Project
	}

	filter := jira4claude.IssueFilter{
		JQL:   fmt.Sprintf("project = %q AND status != Done ORDER BY created DESC", project),
		Limit: c.Limit,
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

	ctx.Printer.Issues(ready)
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
	Parent      string   `help:"Parent issue key (creates a Subtask)" short:"P"`
}

// Run executes the create command.
func (c *IssueCreateCmd) Run(ctx *IssueContext) error {
	project := c.Project
	if project == "" {
		project = ctx.Config.Project
	}

	issueType := c.Type
	if c.Parent != "" {
		issueType = "Subtask"
	}

	// Always convert description as GFM (plain text is valid GFM)
	description := c.Description
	if description != "" {
		adfJSON, err := markdownToADFJSON(description)
		if err != nil {
			return err
		}
		description = adfJSON
	}

	issue := &jira4claude.Issue{
		Project:     project,
		Type:        issueType,
		Summary:     c.Summary,
		Description: description,
		Priority:    c.Priority,
		Labels:      c.Labels,
		Parent:      c.Parent,
	}

	created, err := ctx.Service.Create(context.Background(), issue)
	if err != nil {
		return err
	}

	ctx.Printer.Success("Created:", created.Key)
	return nil
}

// IssueUpdateCmd updates an issue.
type IssueUpdateCmd struct {
	Key         string   `arg:"" help:"Issue key"`
	Summary     *string  `help:"New summary" short:"s"`
	Description *string  `help:"New description" short:"d"`
	Priority    *string  `help:"New priority"`
	Assignee    *string  `help:"New assignee" short:"a"`
	Labels      []string `help:"New labels" short:"l"`
	ClearLabels bool     `help:"Clear all labels" name:"clear-labels"`
}

// Run executes the update command.
func (c *IssueUpdateCmd) Run(ctx *IssueContext) error {
	// Always convert description as GFM (plain text is valid GFM)
	description := c.Description
	if description != nil && *description != "" {
		adfJSON, err := markdownToADFJSON(*description)
		if err != nil {
			return err
		}
		description = &adfJSON
	}

	update := jira4claude.IssueUpdate{
		Summary:     c.Summary,
		Description: description,
		Priority:    c.Priority,
		Assignee:    c.Assignee,
	}

	if len(c.Labels) > 0 {
		update.Labels = &c.Labels
	} else if c.ClearLabels {
		empty := []string{}
		update.Labels = &empty
	}

	updated, err := ctx.Service.Update(context.Background(), c.Key, update)
	if err != nil {
		return err
	}

	ctx.Printer.Success("Updated:", updated.Key)
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
	Key       string `arg:"" help:"Issue key"`
	AccountID string `help:"User account ID (omit to unassign)" short:"a"`
}

// Run executes the assign command.
func (c *IssueAssignCmd) Run(ctx *IssueContext) error {
	if err := ctx.Service.Assign(context.Background(), c.Key, c.AccountID); err != nil {
		return err
	}

	if c.AccountID == "" {
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
	// Always convert body as GFM (plain text is valid GFM)
	body := c.Body
	if body != "" {
		adfJSON, err := markdownToADFJSON(body)
		if err != nil {
			return err
		}
		body = adfJSON
	}

	comment, err := ctx.Service.AddComment(context.Background(), c.Key, body)
	if err != nil {
		return err
	}

	ctx.Printer.Success("Added comment "+comment.ID+" to", c.Key)
	return nil
}

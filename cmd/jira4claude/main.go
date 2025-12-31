package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/lipgloss"
	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/http"
	"github.com/fwojciec/jira4claude/yaml"
)

// CLI defines the command structure for jira4claude.
type CLI struct {
	Config string `help:"Path to config file (auto-discovers if not set)" type:"path"`
	JSON   bool   `help:"Output in JSON format" short:"j"`

	Init       InitCmd       `cmd:"" help:"Initialize a new config file in current directory"`
	Create     CreateCmd     `cmd:"" help:"Create a new issue"`
	View       ViewCmd       `cmd:"" help:"View an issue"`
	List       ListCmd       `cmd:"" help:"List issues"`
	Ready      ReadyCmd      `cmd:"" help:"List issues ready to work on (no unresolved blockers)"`
	Edit       EditCmd       `cmd:"" help:"Edit an issue"`
	Comment    CommentCmd    `cmd:"" help:"Add a comment to an issue"`
	Transition TransitionCmd `cmd:"" help:"Transition an issue to a new status"`
	Assign     AssignCmd     `cmd:"" help:"Assign an issue to a user"`
	Link       LinkCmd       `cmd:"" help:"Link two issues together"`
	Unlink     UnlinkCmd     `cmd:"" help:"Remove link between two issues"`
}

// InitCmd initializes a new config file.
type InitCmd struct {
	Server  string `help:"Jira server URL (e.g., https://example.atlassian.net)" required:""`
	Project string `help:"Default project key (e.g., PROJ)" required:""`
}

// CreateCmd creates a new issue.
type CreateCmd struct {
	Project     string   `help:"Project key" short:"p"`
	Type        string   `help:"Issue type (e.g., Bug, Task, Story)" short:"t" default:"Task"`
	Summary     string   `help:"Issue summary" short:"s" required:""`
	Description string   `help:"Issue description" short:"d"`
	Priority    string   `help:"Issue priority"`
	Labels      []string `help:"Issue labels (can be repeated)" short:"l"`
	Parent      string   `help:"Parent issue key (creates a Subtask)" short:"P"`
}

// ViewCmd views an issue.
type ViewCmd struct {
	Key string `arg:"" help:"Issue key (e.g., PROJ-123)"`
}

// ListCmd lists issues.
type ListCmd struct {
	Project  string   `help:"Filter by project" short:"p"`
	Status   string   `help:"Filter by status" short:"s"`
	Assignee string   `help:"Filter by assignee (use 'me' for current user)" short:"a"`
	Parent   string   `help:"Filter by parent issue (for subtasks)" short:"P"`
	Labels   []string `help:"Filter by labels (must have all)" short:"l"`
	JQL      string   `help:"Raw JQL query (overrides other filters)"`
	Limit    int      `help:"Maximum number of results" default:"50"`
}

// ReadyCmd lists issues ready to work on.
type ReadyCmd struct {
	Project string `help:"Filter by project" short:"p"`
	Limit   int    `help:"Maximum number of results" default:"50"`
}

// EditCmd edits an issue.
type EditCmd struct {
	Key         string   `arg:"" help:"Issue key (e.g., PROJ-123)"`
	Summary     *string  `help:"New summary" short:"s"`
	Description *string  `help:"New description" short:"d"`
	Priority    *string  `help:"New priority"`
	Assignee    *string  `help:"New assignee (empty to unassign)" short:"a"`
	Labels      []string `help:"New labels (replaces existing)" short:"l"`
	ClearLabels bool     `help:"Clear all labels" name:"clear-labels"`
}

// CommentCmd adds a comment to an issue.
type CommentCmd struct {
	Key  string `arg:"" help:"Issue key (e.g., PROJ-123)"`
	Body string `help:"Comment body" short:"b" required:""`
}

// TransitionCmd transitions an issue.
type TransitionCmd struct {
	Key          string `arg:"" help:"Issue key (e.g., PROJ-123)"`
	Status       string `help:"Target status name" short:"s"`
	TransitionID string `help:"Transition ID (use instead of status name)" short:"i"`
	ListOnly     bool   `help:"List available transitions without executing" short:"l"`
}

// AssignCmd assigns an issue.
type AssignCmd struct {
	Key       string `arg:"" help:"Issue key (e.g., PROJ-123)"`
	AccountID string `help:"User account ID (omit to unassign)" short:"a"`
}

// LinkCmd links two issues together.
type LinkCmd struct {
	InwardKey  string `arg:"" help:"Source issue key (e.g., PROJ-123)"`
	LinkType   string `arg:"" help:"Link type (e.g., Blocks, Clones, Relates)"`
	OutwardKey string `arg:"" help:"Target issue key (e.g., PROJ-456)"`
}

// UnlinkCmd removes a link between two issues.
type UnlinkCmd struct {
	Key1 string `arg:"" help:"First issue key (e.g., PROJ-123)"`
	Key2 string `arg:"" help:"Second issue key (e.g., PROJ-456)"`
}

// Styles for output formatting.
var (
	keyStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errorStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	priorityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	headerStyle   = lipgloss.NewStyle().Bold(true).Underline(true)
)

// App holds the application state.
type App struct {
	config  *jira4claude.Config
	service jira4claude.IssueService
	jsonOut bool
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("jira4claude"),
		kong.Description("A minimal Jira CLI for AI agents"),
		kong.UsageOnError(),
	)

	// Init command doesn't need config
	if ctx.Command() == "init" {
		if err := ctx.Run(&cli); err != nil {
			printError(cli.JSON, err)
			os.Exit(1)
		}
		return
	}

	app, err := newApp(cli.Config, cli.JSON)
	if err != nil {
		printError(cli.JSON, err)
		os.Exit(1)
	}

	err = ctx.Run(app)
	if err != nil {
		printError(cli.JSON, err)
		os.Exit(1)
	}
}

func newApp(configPath string, jsonOut bool) (*App, error) {
	// Auto-discover config if not specified
	if configPath == "" {
		workDir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath, err = yaml.DiscoverConfig(workDir, homeDir)
		if err != nil {
			return nil, err
		}
	} else if strings.HasPrefix(configPath, "~/") {
		// Expand ~ in explicit config path
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(home, configPath[2:])
	}

	cfg, err := yaml.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	client, err := http.NewClient(cfg.Server)
	if err != nil {
		return nil, err
	}

	return &App{
		config:  cfg,
		service: http.NewIssueService(client),
		jsonOut: jsonOut,
	}, nil
}

// Run executes the create command.
func (c *CreateCmd) Run(app *App) error {
	project := c.Project
	if project == "" {
		project = app.config.Project
	}

	issueType := c.Type
	if c.Parent != "" {
		issueType = "Subtask"
	}

	issue := &jira4claude.Issue{
		Project:     project,
		Type:        issueType,
		Summary:     c.Summary,
		Description: c.Description,
		Priority:    c.Priority,
		Labels:      c.Labels,
		Parent:      c.Parent,
	}

	created, err := app.service.Create(context.Background(), issue)
	if err != nil {
		return err
	}

	if app.jsonOut {
		return printJSON(issueToMap(created, app.config.Server))
	}

	fmt.Printf("Created: %s\n", keyStyle.Render(created.Key))
	fmt.Printf("%s/browse/%s\n", app.config.Server, created.Key)
	return nil
}

// Run executes the view command.
func (c *ViewCmd) Run(app *App) error {
	issue, err := app.service.Get(context.Background(), c.Key)
	if err != nil {
		return err
	}

	if app.jsonOut {
		return printJSON(issueToMap(issue, app.config.Server))
	}

	printIssueDetail(os.Stdout, issue, app.config.Server)
	return nil
}

// Run executes the list command.
func (c *ListCmd) Run(app *App) error {
	filter := jira4claude.IssueFilter{
		Project:  c.Project,
		Status:   c.Status,
		Assignee: c.Assignee,
		Parent:   c.Parent,
		Labels:   c.Labels,
		JQL:      c.JQL,
		Limit:    c.Limit,
	}

	// Use default project if not specified and not using JQL
	if filter.Project == "" && filter.JQL == "" {
		filter.Project = app.config.Project
	}

	issues, err := app.service.List(context.Background(), filter)
	if err != nil {
		return err
	}

	if app.jsonOut {
		result := make([]map[string]any, len(issues))
		for i, issue := range issues {
			result[i] = issueToMap(issue, app.config.Server)
		}
		return printJSON(result)
	}

	if len(issues) == 0 {
		fmt.Println("No issues found")
		return nil
	}

	printIssueTable(issues)
	return nil
}

// Run executes the ready command.
func (c *ReadyCmd) Run(app *App) error {
	project := c.Project
	if project == "" {
		project = app.config.Project
	}

	// List open issues (status != Done)
	filter := jira4claude.IssueFilter{
		JQL:   fmt.Sprintf("project = %s AND status != Done ORDER BY created DESC", project),
		Limit: c.Limit,
	}

	issues, err := app.service.List(context.Background(), filter)
	if err != nil {
		return err
	}

	// Filter to only ready issues (no unresolved blockers)
	ready := make([]*jira4claude.Issue, 0, len(issues))
	for _, issue := range issues {
		if jira4claude.IsReady(issue) {
			ready = append(ready, issue)
		}
	}

	if app.jsonOut {
		result := make([]map[string]any, len(ready))
		for i, issue := range ready {
			result[i] = issueToMap(issue, app.config.Server)
		}
		return printJSON(result)
	}

	if len(ready) == 0 {
		fmt.Println("No ready issues found")
		return nil
	}

	printIssueTable(ready)
	return nil
}

// Run executes the edit command.
func (c *EditCmd) Run(app *App) error {
	update := jira4claude.IssueUpdate{
		Summary:     c.Summary,
		Description: c.Description,
		Priority:    c.Priority,
		Assignee:    c.Assignee,
	}

	if len(c.Labels) > 0 {
		update.Labels = &c.Labels
	} else if c.ClearLabels {
		empty := []string{}
		update.Labels = &empty
	}

	updated, err := app.service.Update(context.Background(), c.Key, update)
	if err != nil {
		return err
	}

	if app.jsonOut {
		return printJSON(issueToMap(updated, app.config.Server))
	}

	fmt.Printf("Updated: %s\n", keyStyle.Render(updated.Key))
	return nil
}

// Run executes the comment command.
func (c *CommentCmd) Run(app *App) error {
	comment, err := app.service.AddComment(context.Background(), c.Key, c.Body)
	if err != nil {
		return err
	}

	if app.jsonOut {
		return printJSON(map[string]any{
			"id":      comment.ID,
			"author":  comment.Author.DisplayName,
			"body":    comment.Body,
			"created": comment.Created,
		})
	}

	fmt.Printf("Added comment %s to %s\n", comment.ID, keyStyle.Render(c.Key))
	return nil
}

// Run executes the transition command.
func (c *TransitionCmd) Run(app *App) error {
	transitions, err := app.service.Transitions(context.Background(), c.Key)
	if err != nil {
		return err
	}

	if c.ListOnly {
		if app.jsonOut {
			result := make([]map[string]any, len(transitions))
			for i, t := range transitions {
				result[i] = map[string]any{"id": t.ID, "name": t.Name}
			}
			return printJSON(result)
		}

		fmt.Printf("Available transitions for %s:\n", keyStyle.Render(c.Key))
		for _, t := range transitions {
			fmt.Printf("  %s: %s\n", t.ID, statusStyle.Render(t.Name))
		}
		return nil
	}

	if c.TransitionID == "" && c.Status == "" {
		return &jira4claude.Error{
			Code:    jira4claude.EValidation,
			Message: "either --status or --transition-id is required",
		}
	}

	// Find transition by ID or name
	var transitionID string
	if c.TransitionID != "" {
		transitionID = c.TransitionID
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
				available[i] = t.Name
			}
			return &jira4claude.Error{
				Code:    jira4claude.EValidation,
				Message: fmt.Sprintf("status %q not found; available: %s", c.Status, strings.Join(available, ", ")),
			}
		}
	}

	if err := app.service.Transition(context.Background(), c.Key, transitionID); err != nil {
		return err
	}

	if app.jsonOut {
		return printJSON(map[string]any{"key": c.Key, "transitioned": true})
	}

	fmt.Printf("Transitioned %s\n", keyStyle.Render(c.Key))
	return nil
}

// Run executes the assign command.
func (c *AssignCmd) Run(app *App) error {
	if err := app.service.Assign(context.Background(), c.Key, c.AccountID); err != nil {
		return err
	}

	if app.jsonOut {
		if c.AccountID == "" {
			return printJSON(map[string]any{"key": c.Key, "unassigned": true})
		}
		return printJSON(map[string]any{"key": c.Key, "assigned": c.AccountID})
	}

	if c.AccountID == "" {
		fmt.Printf("Unassigned %s\n", keyStyle.Render(c.Key))
	} else {
		fmt.Printf("Assigned %s to %s\n", keyStyle.Render(c.Key), c.AccountID)
	}
	return nil
}

// Run executes the link command.
func (c *LinkCmd) Run(app *App) error {
	if err := app.service.Link(context.Background(), c.InwardKey, c.LinkType, c.OutwardKey); err != nil {
		return err
	}

	if app.jsonOut {
		return printJSON(map[string]any{
			"linked":     true,
			"inwardKey":  c.InwardKey,
			"linkType":   c.LinkType,
			"outwardKey": c.OutwardKey,
		})
	}

	fmt.Printf("Linked %s %s %s\n",
		keyStyle.Render(c.InwardKey),
		c.LinkType,
		keyStyle.Render(c.OutwardKey))
	return nil
}

// Run executes the unlink command.
func (c *UnlinkCmd) Run(app *App) error {
	if err := app.service.Unlink(context.Background(), c.Key1, c.Key2); err != nil {
		return err
	}

	if app.jsonOut {
		return printJSON(map[string]any{
			"unlinked": true,
			"key1":     c.Key1,
			"key2":     c.Key2,
		})
	}

	fmt.Printf("Unlinked %s and %s\n",
		keyStyle.Render(c.Key1),
		keyStyle.Render(c.Key2))
	return nil
}

// Run executes the init command.
func (c *InitCmd) Run(cli *CLI) error {
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

	if cli.JSON {
		return printJSON(map[string]any{
			"configCreated":   result.ConfigCreated,
			"gitignoreAdded":  result.GitignoreAdded,
			"gitignoreExists": result.GitignoreExists,
		})
	}

	if result.ConfigCreated {
		fmt.Println("Created .jira4claude.yaml")
	}
	if result.GitignoreAdded {
		fmt.Println("Added .jira4claude.yaml to .gitignore")
	} else if result.GitignoreExists {
		fmt.Println(".jira4claude.yaml already in .gitignore")
	}
	return nil
}

// Helper functions

func printError(jsonOut bool, err error) {
	if jsonOut {
		_ = printJSON(map[string]any{
			"error":   true,
			"code":    jira4claude.ErrorCode(err),
			"message": jira4claude.ErrorMessage(err),
		})
		return
	}
	fmt.Fprintln(os.Stderr, errorStyle.Render("Error: ")+jira4claude.ErrorMessage(err))
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func issueToMap(issue *jira4claude.Issue, server string) map[string]any {
	m := map[string]any{
		"key":         issue.Key,
		"project":     issue.Project,
		"summary":     issue.Summary,
		"description": issue.Description,
		"status":      issue.Status,
		"type":        issue.Type,
		"priority":    issue.Priority,
		"labels":      issue.Labels,
		"parent":      issue.Parent,
		"created":     issue.Created,
		"updated":     issue.Updated,
		"url":         fmt.Sprintf("%s/browse/%s", server, issue.Key),
	}
	if issue.Assignee != nil {
		m["assignee"] = map[string]any{
			"accountId":   issue.Assignee.AccountID,
			"displayName": issue.Assignee.DisplayName,
			"email":       issue.Assignee.Email,
		}
	}
	if issue.Reporter != nil {
		m["reporter"] = map[string]any{
			"accountId":   issue.Reporter.AccountID,
			"displayName": issue.Reporter.DisplayName,
			"email":       issue.Reporter.Email,
		}
	}
	if len(issue.Links) > 0 {
		links := make([]map[string]any, len(issue.Links))
		for i, link := range issue.Links {
			linkMap := map[string]any{
				"id": link.ID,
				"type": map[string]any{
					"name":    link.Type.Name,
					"inward":  link.Type.Inward,
					"outward": link.Type.Outward,
				},
			}
			if link.OutwardIssue != nil {
				linkMap["outwardIssue"] = map[string]any{
					"key":     link.OutwardIssue.Key,
					"summary": link.OutwardIssue.Summary,
					"status":  link.OutwardIssue.Status,
					"type":    link.OutwardIssue.Type,
				}
			}
			if link.InwardIssue != nil {
				linkMap["inwardIssue"] = map[string]any{
					"key":     link.InwardIssue.Key,
					"summary": link.InwardIssue.Summary,
					"status":  link.InwardIssue.Status,
					"type":    link.InwardIssue.Type,
				}
			}
			links[i] = linkMap
		}
		m["links"] = links
	}
	return m
}

func printIssueDetail(w io.Writer, issue *jira4claude.Issue, server string) {
	fmt.Fprintf(w, "%s  %s\n", keyStyle.Render(issue.Key), issue.Summary)
	fmt.Fprintf(w, "Status: %s  Type: %s", statusStyle.Render(issue.Status), issue.Type)
	if issue.Priority != "" {
		fmt.Fprintf(w, "  Priority: %s", priorityStyle.Render(issue.Priority))
	}
	fmt.Fprintln(w)

	if issue.Assignee != nil {
		fmt.Fprintf(w, "Assignee: %s\n", issue.Assignee.DisplayName)
	}
	if issue.Reporter != nil {
		fmt.Fprintf(w, "Reporter: %s\n", issue.Reporter.DisplayName)
	}
	if issue.Parent != "" {
		fmt.Fprintf(w, "Parent: %s\n", keyStyle.Render(issue.Parent))
	}
	if len(issue.Labels) > 0 {
		fmt.Fprintf(w, "Labels: %s\n", labelStyle.Render(strings.Join(issue.Labels, ", ")))
	}
	if len(issue.Links) > 0 {
		fmt.Fprintln(w, "Links:")
		for _, link := range issue.Links {
			if link.OutwardIssue != nil {
				fmt.Fprintf(w, "  %s %s (%s)\n",
					link.Type.Outward,
					keyStyle.Render(link.OutwardIssue.Key),
					link.OutwardIssue.Summary)
			}
			if link.InwardIssue != nil {
				fmt.Fprintf(w, "  %s %s (%s)\n",
					link.Type.Inward,
					keyStyle.Render(link.InwardIssue.Key),
					link.InwardIssue.Summary)
			}
		}
	}

	if issue.Description != "" {
		fmt.Fprintf(w, "\n%s\n", issue.Description)
	}

	fmt.Fprintf(w, "\n%s/browse/%s\n", server, issue.Key)
}

func printIssueTable(issues []*jira4claude.Issue) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, headerStyle.Render("KEY")+"\t"+headerStyle.Render("STATUS")+"\t"+headerStyle.Render("ASSIGNEE")+"\t"+headerStyle.Render("SUMMARY"))
	for _, issue := range issues {
		assignee := "-"
		if issue.Assignee != nil {
			assignee = issue.Assignee.DisplayName
		}
		summary := truncateString(issue.Summary, 50)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			keyStyle.Render(issue.Key),
			statusStyle.Render(issue.Status),
			assignee,
			summary,
		)
	}
	w.Flush()
}

// truncateString truncates a string to maxLen runes, adding "..." if truncated.
// Uses rune-based slicing to handle multi-byte UTF-8 characters safely.
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

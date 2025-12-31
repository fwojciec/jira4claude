// Package main is the entry point for the j4c CLI.
package main

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/gogh"
	"github.com/fwojciec/jira4claude/http"
	"github.com/fwojciec/jira4claude/yaml"
)

// CLI defines the command structure for j4c.
type CLI struct {
	Config string `help:"Path to config file" type:"path"`
	JSON   bool   `help:"Output in JSON format" short:"j"`

	Issue IssueCmd `cmd:"" help:"Issue operations"`
	Link  LinkCmd  `cmd:"" help:"Link operations"`
	Init  InitCmd  `cmd:"" help:"Initialize config file"`
}

// IssueContext provides dependencies for issue commands.
type IssueContext struct {
	Service jira4claude.IssueService
	Printer jira4claude.Printer
	Config  *jira4claude.Config
}

// LinkContext provides dependencies for link commands.
type LinkContext struct {
	Service jira4claude.IssueService
	Printer jira4claude.Printer
	Config  *jira4claude.Config
}

// MessageContext provides dependencies for message-only commands.
type MessageContext struct {
	Printer jira4claude.MessagePrinter
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("j4c"),
		kong.Description("A minimal Jira CLI for AI agents"),
		kong.UsageOnError(),
	)

	// Build IO and printer (ServerURL set later after config is loaded)
	io := gogh.NewIO(os.Stdout, os.Stderr)
	var printer jira4claude.Printer
	var jsonPrinter *gogh.JSONPrinter
	if cli.JSON {
		jsonPrinter = gogh.NewJSONPrinter(io.Out)
		printer = jsonPrinter
	} else {
		printer = gogh.NewTextPrinter(io)
	}

	// Init command doesn't need config
	if ctx.Command() == "init" {
		msgCtx := &MessageContext{Printer: printer}
		if err := ctx.Run(msgCtx); err != nil {
			printer.Error(err)
			os.Exit(1)
		}
		return
	}

	// Load config
	cfg, err := loadConfig(cli.Config)
	if err != nil {
		printer.Error(err)
		os.Exit(1)
	}

	// Set server URL on printers for URL output
	io.ServerURL = cfg.Server
	if jsonPrinter != nil {
		jsonPrinter.SetServerURL(cfg.Server)
	}

	// Build service
	client, err := http.NewClient(cfg.Server)
	if err != nil {
		printer.Error(err)
		os.Exit(1)
	}
	svc := http.NewIssueService(client)

	// Build contexts
	issueCtx := &IssueContext{Service: svc, Printer: printer, Config: cfg}
	linkCtx := &LinkContext{Service: svc, Printer: printer, Config: cfg}

	// Run command
	if err := ctx.Run(issueCtx, linkCtx); err != nil {
		printer.Error(err)
		os.Exit(1)
	}
}

func loadConfig(configPath string) (*jira4claude.Config, error) {
	if configPath == "" {
		workDir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		configPath, err = yaml.DiscoverConfig(workDir, homeDir)
		if err != nil {
			return nil, err
		}
	}
	return yaml.LoadConfig(configPath)
}

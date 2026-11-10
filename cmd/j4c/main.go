// Package main is the entry point for the j4c CLI.
package main

import (
	"io"
	"os"

	"github.com/alecthomas/kong"
	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/http"
	"github.com/fwojciec/jira4claude/json"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/fwojciec/jira4claude/yaml"
)

// Version information set by goreleaser ldflags.
//
//nolint:gochecknoglobals // Set at build time via ldflags
var (
	version = "dev"
	commit  = "none"
)

// CLI defines the command structure for j4c.
type CLI struct {
	Config  string           `help:"Path to config file" type:"path"`
	JSON    bool             `help:"Output in JSON format" short:"j"`
	Version kong.VersionFlag `help:"Show version information"`

	Issue IssueCmd `cmd:"" help:"Issue operations"`
	Link  LinkCmd  `cmd:"" help:"Link operations"`
	Init  InitCmd  `cmd:"" help:"Initialize config file"`
}

// IssueContext provides dependencies for issue commands.
type IssueContext struct {
	Service     jira4claude.IssueService
	UserService jira4claude.UserService
	Printer     jira4claude.Printer
	Converter   jira4claude.Converter
	Config      *jira4claude.Config
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

// ConfigContext provides dependencies for config commands.
type ConfigContext struct {
	Service jira4claude.ConfigService
	Printer jira4claude.MessagePrinter
}

// Deps are the external dependencies of Run. main wires the real
// implementations; tests substitute mocks.
type Deps struct {
	// Version is the text printed by --version.
	Version string

	// LoadConfig loads the config file. An empty path means "discover it".
	LoadConfig func(path string) (*jira4claude.Config, error)

	// NewServices builds the Jira services for a loaded config. It is called
	// only for commands that need Jira, after the config has loaded.
	NewServices func(cfg *jira4claude.Config) (jira4claude.IssueService, jira4claude.UserService, error)

	// ConfigService backs the init command.
	ConfigService jira4claude.ConfigService
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, defaultDeps()))
}

// defaultDeps returns the production dependencies.
func defaultDeps() Deps {
	return Deps{
		Version:       version + " (" + commit + ")",
		LoadConfig:    loadConfig,
		NewServices:   newHTTPServices,
		ConfigService: yaml.NewService(),
	}
}

// Run executes the CLI with the given arguments (without the program name)
// and streams, and returns the process exit code. The stdin parameter is
// unnamed because no command reads standard input yet; it is part of the
// signature so that callers and tests already pass it.
func Run(args []string, _ io.Reader, stdout, stderr io.Writer, deps Deps) int {
	var cli CLI
	ctx, code, exited := parse(&cli, args, stdout, stderr, deps.Version)
	if exited {
		return code
	}

	// Build printer (ServerURL set later after config is loaded)
	var printer jira4claude.Printer
	var jsonPrinter *json.Printer
	var mdPrinter *markdown.Printer
	if cli.JSON {
		jsonPrinter = json.NewPrinterWithIO(stdout, stderr)
		printer = jsonPrinter
	} else {
		mdPrinter = markdown.NewPrinterWithIO(stdout, stderr)
		printer = mdPrinter
	}

	// Init command doesn't need config
	if ctx.Command() == "init" {
		configCtx := &ConfigContext{
			Service: deps.ConfigService,
			Printer: printer,
		}
		if err := ctx.Run(configCtx); err != nil {
			printer.Error(err)
			return jira4claude.ExitCode(err)
		}
		return 0
	}

	// Load config
	cfg, err := deps.LoadConfig(cli.Config)
	if err != nil {
		printer.Error(err)
		return jira4claude.ExitCode(err)
	}

	// Set server URL on printers for URL output
	if jsonPrinter != nil {
		jsonPrinter.SetServerURL(cfg.Server)
	}
	if mdPrinter != nil {
		mdPrinter.SetServerURL(cfg.Server)
	}

	// Build services
	svc, userSvc, err := deps.NewServices(cfg)
	if err != nil {
		printer.Error(err)
		return jira4claude.ExitCode(err)
	}

	// Build contexts
	conv := markdown.New()
	issueCtx := &IssueContext{Service: svc, UserService: userSvc, Printer: printer, Converter: conv, Config: cfg}
	linkCtx := &LinkContext{Service: svc, Printer: printer, Config: cfg}

	// Run command
	if err := ctx.Run(issueCtx, linkCtx); err != nil {
		printer.Error(err)
		return jira4claude.ExitCode(err)
	}
	return 0
}

// kongExit carries an exit code out of kong's Exit hook.
type kongExit struct {
	code int
}

// parse parses args into cli. Kong terminates the process itself on parse
// errors, --help and --version, and expects its Exit hook not to return. The
// hook here panics with kongExit instead, which parse recovers and reports as
// exited=true with kong's exit code, so Run can return it.
func parse(cli *CLI, args []string, stdout, stderr io.Writer, versionText string) (ctx *kong.Context, code int, exited bool) {
	parser, err := kong.New(cli,
		kong.Name("j4c"),
		kong.Description("A minimal Jira CLI for AI agents"),
		kong.UsageOnError(),
		kong.Vars{"version": versionText},
		kong.Writers(stdout, stderr),
		kong.Exit(func(code int) { panic(kongExit{code: code}) }),
	)
	if err != nil {
		// The CLI grammar is static, so this is a programming error (kong.Parse
		// panics here too).
		panic(err)
	}

	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(kongExit)
			if !ok {
				panic(r)
			}
			ctx, code, exited = nil, e.code, true
		}
	}()

	ctx, err = parser.Parse(args)
	parser.FatalIfErrorf(err)
	return ctx, 0, false
}

// newHTTPServices builds the Jira services backed by the HTTP client.
func newHTTPServices(cfg *jira4claude.Config) (jira4claude.IssueService, jira4claude.UserService, error) {
	client, err := http.NewClient(cfg.Server)
	if err != nil {
		return nil, nil, err
	}
	return http.NewIssueService(client), http.NewUserService(client), nil
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

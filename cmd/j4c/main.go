// Package main is the entry point for the j4c CLI.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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
	JSON    bool             `help:"Print data as JSON on stdout; errors and warnings become one JSON object per line on stderr" short:"j"`
	Version kong.VersionFlag `help:"Show version information"`
	Timeout time.Duration    `help:"How long to wait on Jira, e.g. 30s or 2m; 0 waits forever. An API call must be answered in full within it, and a request that times out is not retried; an attachment download or upload has no cap on its total time, only on any silence: it fails when nothing arrives or is sent for this long. A timeout exits 7, and a timed-out write says it may have been applied" default:"30s" placeholder:"DURATION"`

	Issue      IssueCmd      `cmd:"" help:"Issue operations"`
	Link       LinkCmd       `cmd:"" help:"Link operations"`
	Attachment AttachmentCmd `cmd:"" help:"Attachment operations"`
	Init       InitCmd       `cmd:"" help:"Write .jira4claude.yaml (server and default project) in the current directory and add it to .gitignore"`
}

// Validate refuses a negative --timeout.
func (c *CLI) Validate() error {
	if c.Timeout < 0 {
		return &jira4claude.Error{Code: jira4claude.EValidation, Message: "--timeout cannot be negative; use 0 for no timeout"}
	}
	return nil
}

// IssueContext provides dependencies for issue commands.
type IssueContext struct {
	Service     jira4claude.IssueService
	UserService jira4claude.UserService
	Printer     jira4claude.Printer
	Converter   jira4claude.Converter
	Config      *jira4claude.Config
	// Stdin is read by a file flag given "-" (--description-file -).
	Stdin io.Reader
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
	Printer jira4claude.Printer
	// WorkDir is the directory init writes to; empty means the process's
	// working directory.
	WorkDir string
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

	// NewAttachmentService builds the attachment service for a loaded
	// config. It is called only for attachment commands, which do not call
	// NewServices.
	NewAttachmentService func(cfg *jira4claude.Config) (jira4claude.AttachmentService, error)

	// ConfigService backs the init command.
	ConfigService jira4claude.ConfigService
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, defaultDeps()))
}

// defaultDeps returns the production dependencies.
func defaultDeps() Deps {
	return Deps{
		Version:              version + " (" + commit + ")",
		LoadConfig:           loadConfig,
		NewServices:          newHTTPServices,
		NewAttachmentService: newHTTPAttachmentService,
		ConfigService:        yaml.NewService(),
	}
}

// Run executes the CLI with the given arguments (without the program name)
// and streams, and returns the process exit code. stdin is read only by a file
// flag given "-".
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, deps Deps) int {
	var cli CLI
	ctx, code, exited := parse(&cli, args, stdout, stderr, deps.Version)
	if exited {
		return code
	}

	var printer jira4claude.Printer
	if cli.JSON {
		printer = json.NewPrinterWithIO(stdout, stderr)
	} else {
		printer = markdown.NewPrinterWithIO(stdout, stderr)
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
	cfg.Timeout = cli.Timeout

	if strings.HasPrefix(ctx.Command(), "attachment ") {
		return runAttachment(ctx, printer, cfg, deps)
	}

	// Build services
	svc, userSvc, err := deps.NewServices(cfg)
	if err != nil {
		printer.Error(err)
		return jira4claude.ExitCode(err)
	}

	// Build contexts
	conv := markdown.New()
	issueCtx := &IssueContext{Service: svc, UserService: userSvc, Printer: printer, Converter: conv, Config: cfg, Stdin: stdin}
	linkCtx := &LinkContext{Service: svc, Printer: printer, Config: cfg}

	// Run command
	if err := ctx.Run(issueCtx, linkCtx); err != nil {
		printer.Error(err)
		return jira4claude.ExitCode(err)
	}
	return 0
}

// runAttachment runs an attachment command with the attachment service built
// for cfg, and returns the exit code.
func runAttachment(ctx *kong.Context, printer jira4claude.Printer, cfg *jira4claude.Config, deps Deps) int {
	if deps.NewAttachmentService == nil {
		err := &jira4claude.Error{Code: jira4claude.EInternal, Message: "no attachment service is configured"}
		printer.Error(err)
		return jira4claude.ExitCode(err)
	}
	svc, err := deps.NewAttachmentService(cfg)
	if err != nil {
		printer.Error(err)
		return jira4claude.ExitCode(err)
	}
	if err := ctx.Run(&AttachmentContext{Service: svc, Printer: printer, Config: cfg}); err != nil {
		printer.Error(err)
		return jira4claude.ExitCode(err)
	}
	return 0
}

// description is the top-level help text: what the tool is, and the rules a
// script relies on. The README's Scripting section has the details.
const description = "A minimal Jira CLI for AI agents. It never prompts.\n\n" +
	"stdout carries only data. Errors, warnings and notices go to stderr; with --json, " +
	"each is one JSON object per line, with a \"level\" key. " +
	"Exit codes: 0 success, 1 validation or usage error (including a refused write), 2 authentication, " +
	"3 permission, 4 not found, 5 conflict (--if-updated), 6 rate limit, 7 internal (including network errors and timeouts; see --timeout). " +
	"A write that would lose content is refused unless --force.\n\n" +
	"Scripting guide: https://github.com/fwojciec/jira4claude#scripting"

// kongExit carries an exit code out of kong's Exit hook.
type kongExit struct {
	code int
}

// parse parses args into cli. Kong terminates the process itself on --help and
// --version, which print to stdout, and expects its Exit hook not to return.
// The hook here panics with kongExit instead, which parse recovers and reports
// as exited=true with kong's exit code, so Run can return it.
//
// A parse error (unknown flag, missing argument, bad flag value) is a usage
// error: a validation error, reported on stderr. In text mode stderr gets the
// usage text and the parser's message; in --json mode it gets only the message,
// as one JSON error line. stdout stays empty.
func parse(cli *CLI, args []string, stdout, stderr io.Writer, versionText string) (ctx *kong.Context, code int, exited bool) {
	parser, err := kong.New(cli,
		kong.Name("j4c"),
		kong.Description(description),
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
	if err == nil {
		return ctx, 0, false
	}
	usageErr := &jira4claude.Error{Code: jira4claude.EValidation, Message: err.Error()}
	if cli.JSON || requestsJSON(args) {
		json.NewPrinterWithIO(io.Discard, stderr).Error(usageErr)
		return nil, jira4claude.ExitCode(usageErr), true
	}
	var parseErr *kong.ParseError
	if errors.As(err, &parseErr) && parseErr.Context != nil {
		// Kong prints usage to its stdout writer; point it at stderr.
		parser.Stdout = stderr
		// Error ignored: as with the printers, a failing writer has no
		// recovery path, and the error line below still follows.
		_ = parseErr.Context.PrintUsage(false)
		fmt.Fprintln(stderr)
	}
	parser.Errorf("%s", err.Error())
	return nil, jira4claude.ExitCode(usageErr), true
}

// requestsJSON reports whether args ask for --json output. It is consulted
// only when parsing failed: kong stops at the first bad token, so a --json
// after it never reaches the CLI struct. It follows kong's rules: the last
// occurrence wins, --json=VALUE takes true, 1 or yes (any case) as true, and
// arguments after "--" are positional.
func requestsJSON(args []string) bool {
	requested := false
	for _, a := range args {
		if a == "--" {
			break
		}
		if a == "--json" || a == "-j" {
			requested = true
			continue
		}
		if value, ok := strings.CutPrefix(a, "--json="); ok {
			switch strings.ToLower(value) {
			case "true", "1", "yes":
				requested = true
			case "false", "0", "no":
				requested = false
			}
		}
	}
	return requested
}

// newHTTPServices builds the Jira services backed by the HTTP client.
func newHTTPServices(cfg *jira4claude.Config) (jira4claude.IssueService, jira4claude.UserService, error) {
	client, err := http.NewClient(cfg.Server, http.WithTimeout(cfg.Timeout))
	if err != nil {
		return nil, nil, err
	}
	return http.NewIssueService(client), http.NewUserService(client), nil
}

// newHTTPAttachmentService builds the attachment service backed by the HTTP
// client.
func newHTTPAttachmentService(cfg *jira4claude.Config) (jira4claude.AttachmentService, error) {
	client, err := http.NewClient(cfg.Server, http.WithTimeout(cfg.Timeout))
	if err != nil {
		return nil, err
	}
	return http.NewAttachmentService(client), nil
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

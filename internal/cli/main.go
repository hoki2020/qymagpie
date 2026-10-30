// Package cli is pigeon's command line: every command, and the app, tray
// and TUI it opens (the root's main.go only calls Main).
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/hoki2020/pigeon/internal/agent"
	"github.com/hoki2020/pigeon/internal/appdir"
	"github.com/hoki2020/pigeon/internal/catalog"
	"github.com/hoki2020/pigeon/internal/claudebridge"
	"github.com/hoki2020/pigeon/internal/davsync"
	"github.com/hoki2020/pigeon/internal/gateway"
	"github.com/hoki2020/pigeon/internal/imagemcp"
	"github.com/hoki2020/pigeon/internal/netproxy"
	"github.com/hoki2020/pigeon/internal/pluginrt"
	"github.com/hoki2020/pigeon/internal/proc"
	"github.com/hoki2020/pigeon/internal/profile"
	"github.com/hoki2020/pigeon/internal/provider"
	"github.com/hoki2020/pigeon/internal/sessions"
	"github.com/hoki2020/pigeon/internal/settings"
	"github.com/hoki2020/pigeon/internal/tui"
	"github.com/hoki2020/pigeon/internal/update"
)

// version is the root main package's, set at build time
// (-ldflags "-X main.version=…") and handed over by Main.
var version = "dev"

const usage = `pigeon — one place to pick every agent's model

  pigeon                          open the app: a window plus a menu bar icon
  pigeon tray                     start in the menu bar only
  pigeon panel                    open the menu bar icon's quick panel, or close it
  pigeon autostart [on|off]       open pigeon (in the menu bar) when you log in, or say whether it does
  pigeon tui                      the same thing, in the terminal (serves the gateway while open when no pigeon does)
  pigeon web [--addr host:port] [--lan] [--no-open] [--gateway] [--tls-cert <file> --tls-key <file>]
                                  the app's window in a browser, with the gateway (no desktop needed: WSL, a server over SSH)
                                  a new key each run; MAGPIE_WEB_KEY (16+ letters, digits, - . _ ~) keeps one, signed in for 400 days
                                  behind a reverse proxy, MAGPIE_WEB_URL=https://<the page there> prints the link through it
                                  --gateway: gateway mode, no Agents, Sessions or Library (on by itself with no agents here; Settings › General turns it off)
                                  --tls-cert/--tls-key: the page and the gateway over https:// to other computers
  pigeon ls                       list detected agents and their settings
  pigeon <agent>                  show one agent
  pigeon <agent> <model>          set an agent's model   e.g. pigeon claude deepseek/deepseek-chat
  pigeon <agent> <field>          show one field   e.g. pigeon codex effort
  pigeon <agent> <field> <value>  set another field   e.g. pigeon codex effort high
  pigeon <agent> default          take pigeon out: the agent back on what it had before
  pigeon <agent> <field> default  that field back to the agent's own default
  pigeon <command> --help         what a command or an agent takes; -h or --help anywhere does nothing else

  pigeon save <name>              snapshot every agent's settings as a profile
  pigeon use <name>               apply a profile
  pigeon profiles                 list profiles
  pigeon rm <name>                delete a profile

  pigeon backup [--no-keys] [--no-library] [file]    providers, keys, settings, profiles, agent models and the library in one file, sealed with a passphrase
  pigeon restore [--no-agents] [--no-library] <file> put a backup in on this machine
  pigeon webdav [on <address>|set k=v…|now|off]      the same, kept the same on every computer through a WebDAV folder (pigeon webdav help)
  pigeon s3 [on s3://<bucket>[/<prefix>]|set k=v…|now|off]   the same through an S3-compatible bucket: AWS, R2, B2, MinIO… (pigeon s3 help)
  pigeon github [on github://<owner>/<repo>[/<folder>]|set k=v…|now|off]   the same in an encrypted GitHub repository (pigeon github help)

  pigeon library [sync|instructions|mcp|skill]   the instructions, MCP servers and skills written into every agent (pigeon library help)

  pigeon providers                list your providers: host, key, models, who uses them
  pigeon presets                  the vendors pigeon knows: add one with just a key
  pigeon provider add <preset> <key>   e.g. pigeon provider add deepseek sk-…
  pigeon provider add <name> k=v…      a custom vendor (pigeon provider for the fields)
  pigeon provider key|models|test|rm <id>
  pigeon provider fallback <id> <provider/model>…   use these when it's out of quota or down
  pigeon import [-y] <link>       add the provider a magpie://import?… link describes
  pigeon models [<agent>]         every model agents can pick, as provider/model; an agent's, and why others aren't
  pigeon model name <provider/model> <name>|--reset       the name a model goes by, everywhere
  pigeon model efforts <provider/model> <l>,<l>|--reset   the reasoning levels a model offers (pigeon model help)
  pigeon visible [<agent> <family|provider|group>,… | all | --only-picked | --show-new]
                                  which models an agent is shown: families (pigeon provider/group set <id> family=…),
                                  or only the models ticked for it, a new one off until it is ticked
  pigeon search [add <api> <key>|rm <api>]   Tavily, Brave, Exa, Firecrawl, You.com, DeepSeek or SearXNG for web search when no provider can search
  pigeon groups                   routing groups: several models agents pick as one, group/<id>
  pigeon group add <name> models=<m1>,<m2> [routing=smart|order|rotate|usage|pace] [stays=auto|session|turn|off]
  pigeon group <id> | set <id> k=v… | rm <id>   show, change or remove one (pigeon group help for more)
  pigeon accounts [agent] [--json]  every subscription pigeon knows, with each one's allowance used and when it resets
  pigeon accounts add <agent>     sign in to one more Claude, ChatGPT or Google (Gemini CLI, Antigravity) subscription
  pigeon accounts add copilot [--host <name>.ghe.com]   one more Copilot account, on github.com or an enterprise's GHE.com
  pigeon accounts discovery [on|off]   read agents' sign-ins (default on; MAGPIE_NO_ACCOUNTS=1 disables too)
  pigeon accounts import <codex|claude|antigravity|factory> <file>... [--yes]   bring in accounts from other tools' files (Codex's auth.json, Cockpit Tools, CLIProxyAPI, Sub2API); the tool a ChatGPT or Claude file came from is signed out of it
  pigeon accounts switch <agent> <email>   sign the agent in to another of them
  pigeon accounts refresh         renew the saved ChatGPT sign-ins now (the gateway does it daily)
  pigeon accounts checkin         WorkBuddy's daily check-in (签到) for each WorkBuddy account, now (Settings can do it daily)
  pigeon accounts project <gemini|antigravity> <email> <project>   the Google Cloud project a Google account's requests go to
  pigeon accounts alias <agent> <email> [<name>|--clear]   a name of your own for an account, shown on its card, in the menu bar and in pigeon quota
  pigeon accounts switch-only codex [on|off]   only switch Codex's own sign-in to an account with room; its requests go straight to OpenAI, never through pigeon
  pigeon claude-code [install [--yes]|remove]   the Claude Code a Claude subscription runs; install downloads Anthropic's own build, checked against its manifest, for a server or container without one
  pigeon plugin [add <package>|rm|update|on|off|login <provider>|logout <provider>]
                                  OpenCode provider plugins and pi packages: subscriptions signed in to, and served, through a plugin
  pigeon plugin move|migrate <subscription>   run a built-in subscription's accounts on its community plugin

  pigeon serve [--web[=host:port]] [--tls-cert <file> --tls-key <file>]
                                  run the gateway alone (the app runs it too); --web serves pigeon web's page too, from the same process (127.0.0.1:3430 unless told);
                                  with a certificate and key (PEM; MAGPIE_TLS_CERT, MAGPIE_TLS_KEY) other computers reach both over https://
  pigeon daemon [install|uninstall|start|stop|restart|status]
                                  pigeon serve as a service of yours, from boot or login and started again when it stops (systemd --user on Linux, launchd on the Mac)
  pigeon daemon install [--web] [--lan] [--addr host:port] [--no-web] [--tls-cert <file> --tls-key <file>|--no-tls]
                                  --web: the service serves pigeon web's page too, with a key kept across restarts; installed again, it keeps what it served
  pigeon daemon web [on|off] [--lan] [--addr host:port]
                                  the service's page: its link, or turn it on or off
  pigeon healthcheck              exit 0 when the gateway answers (a container's HEALTHCHECK)
  pigeon heap [<file>]            a Go heap profile of the running pigeon, with its memory numbers, for a report of it using too much memory
  pigeon cpu [<file>] [--seconds N]   a Go CPU profile of the running pigeon over 30s (5..120) while you use it, for a report of it using too much CPU
  pigeon goroutines [<file>]      every goroutine of the running pigeon, its stack and how long it waited, for a report of it stuck at something
  pigeon gateway-key list|add <name>|rotate <id>|remove <id>   manage the keys clients use to call a shared gateway
  pigeon gateway-key limit <id> [list|off|reset [n]|remove <n>|[add] day|week|month|<N>d|<N>h --tokens N --cost USD --cache-reads]   a key's own limits, and what it used
  pigeon gateway-key models <id> [all|<provider>/<model>|<provider>/* ...]   the models a key may use, every one unless it names some
  pigeon gateway-key quotas <id> [on|off]   whether a key held to a budget, models or accounts is told the quotas of what it may use
  pigeon mcp image                the image and video generation MCP server an agent is given from the library (stdio)
  pigeon usage [today|7d|30d|all] tokens and cost per agent, model and subscription account (30d)
  pigeon usage --csv [--account <name>] [today|7d|30d|all]   every request as CSV (or one account's): the model asked for, sent and served, tokens, cost, time, status, account
  pigeon usage [--csv [--account <name>]] --last <dur>|--days <n>   the same for a window ending now (24h, 90m, 1.5 days), not from a midnight
  pigeon sessions [--model <m>] [--folder <f>] [--json]   the latest Claude Code, Codex, OpenCode and Pi sessions, with what each cost
  pigeon sessions --days N|today|all [--model <m>] [--folder <f>] [--json]
                                  what every session spent, day by day, with the top models and folders (7 days)
  pigeon quota [<provider>] [--json]  what is left of every subscription, plan and key balance
  pigeon quota wait <provider|account> [--timeout <d>] [--quiet]
                                  block until that subscription (any of its accounts) or account has allowance again
  pigeon quota history [<provider|account>] [--days N] [--json]
                                  each window's readings over time, kept 45 days
  pigeon statusline [<provider|account>]   one line for Claude Code's status line: the session's model and each account's windows
  pigeon sync                     refresh the model catalog and vendor model lists
  pigeon agents                   list every supported agent
  pigeon update [check] [--proxy <url>] [--mirror <prefix>]
                                  install the newest release (check: only say if there is one); --proxy: an
                                  http(s):// or socks5:// proxy for it; --mirror: a GitHub download mirror put
                                  before the github.com URL (none unless given; still checked against usemagpie.ai's SHA-256)
  pigeon update mirror [<prefix>|off]  the mirror every update, the app's own too, is downloaded through
  pigeon update auto [on|off] [download|notify] [30m|1h|6h|24h]
                                  whether the app looks for updates by itself, how often (6h), and whether
                                  it downloads one at once or only says it is out (notify: for metered networks)

agents: claude (cc), codex, gemini, opencode (oc), mimocode, deveco, pi, goose, cursor, zed, copilot, crush, aside
`

var (
	bold  = lipgloss.NewStyle().Bold(true)
	muted = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8B8F98", Dark: "#7C8290"})
	faint = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#C4C7CE", Dark: "#4A4F5A"})
	green = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#0F9D58", Dark: "#7EE2A8"})
)

// Main runs pigeon with os.Args, at the version the build was stamped with.
func Main(v string) {
	version = v
	// before anything reads or writes a file: no home, or a relative one,
	// would put the agents' configs and pigeon's keys under the working
	// folder. What needs no file still answers (pigeon version in a
	// container or a script without HOME): run makes pigeon's folders first.
	ignored, err := appdir.CheckEnv()
	if err != nil {
		if len(os.Args) > 1 {
			switch os.Args[1] {
			case "-v", "--version", "version":
				fmt.Println("pigeon", version)
				return
			case "-h", "--help", "help":
				fmt.Print(usage)
				return
			}
		}
		fmt.Fprintln(os.Stderr, "pigeon:", err)
		os.Exit(1)
	}
	slices.Sort(ignored)
	for _, v := range ignored {
		fmt.Fprintf(os.Stderr, "pigeon: ignoring %s: not an absolute path (the programs pigeon starts still get it)\n", v)
	}
	if provider.TookOpenedURL(os.Args[1:]) {
		// Claude Code, signing in for pigeon, handed over the page to open
		return
	}
	endProbesOnSignal()
	gateway.Version = version
	netproxy.Install()
	update.GUI = hasGUI
	err = run(os.Args[1:])
	proc.EndProbes() // a CLI still being asked something isn't left to init
	sessions.Saved() // the session index kept, for the next run
	if err != nil {
		// a command with exit codes of its own (quota wait) says which
		code := 1
		var e exitError
		if errors.As(err, &e) {
			code = e.code
		}
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(os.Stderr, "pigeon:", msg)
		}
		os.Exit(code)
	}
}

// runTUI runs the TUI, which quits on Ctrl+C and SIGTERM itself once it
// has started; it asks CLIs first, and a signal then ends those.
func runTUI() error {
	return tuiRun(ownSignals)
}

// tuiRun is tui.Run; a var so tests can stand in for it.
var tuiRun = tui.Run

// syncAgentsOnChange sets what a catalog change (a provider or group
// saved, a vendor's list fetched anew) does: the model lists agents keep
// in files of their own are brought up to date behind the save, which
// answers without waiting for every agent's files to be written (on
// Windows, half a second for each one another program holds open).
func syncAgentsOnChange() { catalog.Changed = agent.SyncCatalogSoon }

// startUp brings the files pigeon and its agents keep up to what this
// pigeon reads, before a command that may read them; a var so a test can
// tell it ran.
var startUp = func() {
	// started by an update, the pigeon it replaces goes first: the moves
	// below write the files it may still be writing
	update.AwaitPredecessor()
	makeDirs()
	settings.Migrate()
	// every built-in plugin starts on (plugins.yml), on a new install too
	if _, err := pluginrt.Settle(); err != nil {
		fmt.Fprintf(os.Stderr, "pigeon: plugins.yml: %v\n", err)
	}
	if _, err := pluginrt.Refresh(); err != nil {
		fmt.Fprintf(os.Stderr, "pigeon: plugins.yml: %v\n", err)
	}
	// the providers and settings read once for every agent's fields, which
	// the moves below look at (a write among them reads them again)
	release := provider.Hold()
	agent.RenameLegacy()
	agent.MoveCursorEfforts()
	agent.MoveAntigravityEfforts()
	agent.MoveOffAccountIDs()
	release()
}

func run(args []string) error {
	// a command that saved a provider or a group ends with the agents'
	// files written, which its save no longer waits for; and a test's run
	// leaves catalog.Changed as it found it
	was := catalog.Changed
	defer func() {
		agent.SyncCatalogSettled()
		catalog.Changed = was
	}()
	// before anything is read, moved or started: --help on any command
	// prints its usage and does nothing else
	if helpAsked(args) {
		fmt.Println(strings.TrimRight(helpFor(args), "\n"))
		return nil
	}
	if len(args) > 0 && args[0] == "healthcheck" {
		return healthcheck() // every few seconds in a container: nothing else
	}
	if len(args) > 0 && args[0] == "heap" {
		// the running pigeon's heap profile: asks it, and moves nothing here
		return heapCmd(args[1:])
	}
	if len(args) > 0 && args[0] == "cpu" {
		// the running pigeon's CPU profile: asks it, and moves nothing here
		return cpuCmd(args[1:])
	}
	if len(args) > 0 && args[0] == "goroutines" {
		// the running pigeon's goroutines: asks it, and moves nothing here
		return goroutinesCmd(args[1:])
	}
	if len(args) == 2 && args[0] == agent.DryRunArg {
		// an agent disconnected on a copy of its files under a temporary
		// home, for the Agents page to show what disconnecting changes
		// (agent.DisconnectPreview)
		return agent.DryRun(args[1])
	}
	if len(args) > 1 && (args[0] == "accounts" || args[0] == "account") && args[1] == "discovery" {
		return accountsDiscoveryCmd(args[2:])
	}
	// what an agent starts for each session or message talks to the
	// running gateway and nothing else: the moves below list every
	// provider and account, which starts the plugin host and asks claude,
	// cursor-agent and devin who is signed in — five processes and a
	// Claude /usage for each Claude Code session's image MCP (#3318)
	if len(args) > 0 {
		switch args[0] {
		case "mcp":
			return imagemcp.Run(args[1:])
		case "claude-mcp-helper": // internal: stdio MCP subprocess spawned by Claude Code
			return claudebridge.RunMCP(args[1:])
		case "statusline":
			return statuslineCmd(args)
		}
	}
	startUp()
	// a provider added, edited or removed, or a list fetched anew, reaches
	// the model lists agents keep in files of their own, behind the save
	// that changed it (agent.SyncCatalogSoon)
	syncAgentsOnChange()
	// a model Claude Code names that pigeon doesn't serve goes to the one
	// it is set to use for that tier
	gateway.StandIn = agent.StandIn
	// and its auto mode classifier, on a sonnet tier that follows a main
	// model /model has moved, to the model it moved to
	gateway.ClassifierStandIn = agent.ClassifierStandIn
	// a Codex that reaches pigeon for account failover alone is handed
	// only its own models (#1385)
	gateway.CodexOwnOnly = agent.CodexOwnOnly
	// the Codex app's titles, descriptions and summaries, which it asks
	// for on a model of its own, on the model of pigeon's picked beside
	// its ChatGPT sign-in rather than on that account (atom_Ray on X)
	gateway.CodexSideModel = agent.CodexSideModel
	// the setup kept the same on every computer, by whichever serves
	gateway.WhileServing = append(gateway.WhileServing, davsync.Run)
	// and dsh's patch lists, which dsh reads live: a route left behind by
	// something else writing the file fails every session there until
	// pigeon writes its own list again
	gateway.WhileServing = append(gateway.WhileServing, agent.KeepDshWired)
	// and Cursor Private Inference's variables, which the Mac's launchd
	// forgets at a restart, for the gateway's address now
	gateway.WhileServing = append(gateway.WhileServing, agent.KeepCursorLocalEnv)
	// and Claude Science's daemon, which a restart of the computer ends,
	// started on the gateway again while it is connected
	gateway.WhileServing = append(gateway.WhileServing, agent.KeepClaudeScience)
	// and the request archive, when it is on, goes to the bucket sync is to
	gateway.ArchiveBucket = func() (gateway.Putter, bool) {
		if b, ok := davsync.S3Bucket(); ok {
			return b, true
		}
		return nil, false
	}
	if len(args) == 0 {
		if hasGUI {
			return runGUI(true, "")
		}
		gateway.Serves = "tui"
		return runTUI()
	}
	// a magpie:// link the system handed over (Windows, Linux): the app
	// opens it for the user to confirm
	if strings.HasPrefix(strings.ToLower(args[0]), "pigeon:") {
		if !hasGUI {
			return importCmd(args)
		}
		return runGUI(false, args[0])
	}
	// how this process was started, for the Routing page of another pigeon on
	// this computer to name the one that serves the gateway (#2606)
	switch args[0] {
	case "tui", "web", "serve", "tray":
		gateway.Serves = args[0]
	case "app", "gui", "panel", "-Embedding":
		gateway.Serves = "app"
	}
	switch args[0] {
	case "tui":
		return runTUI()
	case "web":
		return webCmd(args[1:])
	case "app", "gui":
		// `pigeon gui settings`: the window on that tab, as a restart to
		// update from it comes back (update.RelaunchArgs)
		if len(args) > 1 {
			return runWindow(args[1])
		}
		return runGUI(true, "")
	case "tray":
		return runGUI(false, "")
	case "-Embedding":
		// Windows starting pigeon for a click on one of its notifications
		// (a usage alert, #368) left in the Action Center after it quit:
		// the window, on the Usage page
		return runWindow("usage")
	case "panel":
		return runPanel()
	case "autostart":
		return autostartCmd(args[1:])
	case "daemon":
		return daemonCmd(args[1:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return nil
	case "-v", "--version", "version":
		fmt.Println("pigeon", version)
		return nil
	case "ls", "list":
		// in the order the app lists them; those hidden there come last, dimmed
		shown, hidden := settings.Arrange(settings.Load(), agent.Detected(), func(a *agent.Agent) string { return a.ID })
		return list(append(shown, hidden...), true, len(shown))
	case "agents":
		return list(agent.All(), false, -1)
	case "sync":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := catalog.Sync(ctx); err != nil {
			return err
		}
		fmt.Println(green.Render("✓"), "catalog saved to", catalog.CachePath())
		refreshLive(ctx)
		return nil
	case "save", "use", "rm", "profiles":
		return profiles(args)
	case "import":
		return importCmd(args[1:])
	case "providers":
		return providers()
	case "presets":
		return presets()
	case "provider":
		return providerCmd(args)
	case "models":
		return models(args[1:])
	case "model":
		return modelCmd(args[1:])
	case "visible":
		return visibleCmd(args[1:])
	case "search":
		return searchCmd(args[1:])
	case "groups":
		return groups()
	case "group":
		return groupCmd(args)
	case "serve":
		return serveCmd(args[1:])
	case "gateway-key":
		return gatewayKeys(args)
	case "accounts", "account":
		return accountsCmd(args)
	case "usage":
		return usageCmd(args)
	case "sessions":
		return sessionsCmd(args)
	case "quota", "quotas":
		return quotaCmd(args)
	case "update":
		return updateCmd(args)
	case "library", "lib":
		return libraryCmd(args)
	case "backup":
		return backupCmd(args[1:])
	case "restore":
		return restoreCmd(args[1:])
	case "webdav", "dav":
		return webdavCmd(args[1:])
	case "plugin", "plugins":
		return pluginCmd(args)
	case "s3":
		return s3Cmd(args[1:])
	case "github":
		return githubCmd(args[1:])
	case "claude-code":
		return claudeCodeCmd(args[1:])
	}

	a, err := agent.Find(args[0])
	if err != nil {
		return err
	}
	// no agent form takes a flag: one is never set as a model or a value
	// (`pigeon snow --verbose` would take pigeon out of Snow, #1457)
	if f := flagArg(args[1:]); f != "" {
		return fmt.Errorf("unknown flag %s: nothing was changed\n\n%s", f, strings.TrimRight(agentHelp(a), "\n"))
	}
	if len(a.Fields) == 0 && a.Import != nil {
		// `pigeon cindy`: it takes pigeon through its own link, confirmed there
		if len(args) > 1 {
			link := a.Import()
			if link == "" {
				// Lody takes only a gateway on this computer's loopback
				return fmt.Errorf("%s takes pigeon only at a loopback address, and the gateway is at %s: nothing was opened", a.Name, gateway.URL())
			}
			if a.ID == "lody" {
				// Lody on Linux is run with the link (agent.OpenLody)
				_ = agent.OpenLody(link, func(u string) error { openInBrowser(u); return nil })
			} else {
				openInBrowser(link)
			}
			fmt.Println(green.Render("✓"), bold.Render(a.Name), muted.Render("opened to add pigeon — confirm it there"))
			fmt.Println(muted.Render("  " + link))
			return nil
		}
	}
	switch len(args) {
	case 1:
		return list([]*agent.Agent{a}, true, -1)
	case 2:
		// default is pigeon's word in any case, as an agent's name and a
		// field's are: pigeon codex DEFAULT wrote model = "DEFAULT"
		if strings.EqualFold(args[1], "default") {
			if a.Wired() {
				return disconnect(a)
			}
			return set(a, a.Fields[0].Key, "")
		}
		// `pigeon codex model`: a field's name alone asks what it is set to,
		// where it was set as the model (model = "model"). Claude Code's
		// opus is a model's name as well as a tier's, and stays the model
		if f := a.Field(args[1]); f != nil && !modelAlias(a, args[1]) {
			return showField(a, f)
		}
		// `pigeon codex xhigh`: a bare value that belongs to a non-model field
		// (effort levels, for instance) is routed there; anything else is a model.
		if f := fieldForValue(a, args[1]); f != nil {
			return set(a, f.Key, args[1])
		}
		return set(a, a.Fields[0].Key, args[1])
	case 3:
		value := args[2]
		if strings.EqualFold(value, "default") {
			value = ""
		}
		return set(a, args[1], value)
	}
	return fmt.Errorf("too many arguments\n\n%s", usage)
}

func set(a *agent.Agent, key, value string) error {
	f := a.Field(key)
	if f == nil {
		var keys []string
		for _, f := range a.Fields {
			keys = append(keys, f.Key)
		}
		return fmt.Errorf("%s has no field %q (fields: %s)", a.Name, key, strings.Join(keys, ", "))
	}
	// a field's name is no value: pigeon codex subagent effort ran Codex's
	// subagents on a model called effort. Claude Code's opus is a model's
	// name as well as a tier's
	if g := a.Field(value); value != "" && g != nil && !modelAlias(a, value) {
		return fieldAsValue(a, f, g, value)
	}
	value, err := a.Spell(f.Key, value)
	if err != nil {
		return err
	}
	before := f.Get()
	wired := a.Wired()
	// a pigeon model picked on an agent not connected yet connects it
	// first, as the Agents page's picker does (#1900): the agent is wired
	// in, and Disconnect puts back what it had before
	if err := a.Pick(f.Key, value); err != nil {
		return err
	}
	// what the config reads now, not what was asked: an agent may name the
	// model under a provider of its own (OpenCode's pigeon-relay/…), and a
	// value it had already is said to be so
	now := f.Get()
	shown := now
	if value == "" || now == "" {
		shown = muted.Render("default")
	}
	if now == before {
		shown += " " + muted.Render("(unchanged)")
	}
	fmt.Println(green.Render("✓"), bold.Render(a.Name), muted.Render(f.Label), shown)
	// a value that is the agent's own, not one of pigeon's, takes pigeon
	// out: said, as Disconnect is, with the way back (#2000: pigeon claude
	// sonnet put Claude Code back on its DeepSeek endpoint, which refused
	// Sonnet, and only "✓ Claude Code model sonnet" was printed)
	if wired && now != "" && !a.Wired() {
		fmt.Println(muted.Render(fmt.Sprintf("  disconnected from pigeon: %s is %s's own, sent to the endpoint it had before pigeon; pigeon %s <model> with one of pigeon's models connects it again", now, a.Name, a.ID)))
	}
	if a.Notice != nil && (!f.Live || !wired && a.Wired()) {
		if n := a.Notice(); n != "" {
			fmt.Println(muted.Render("  ↻ " + n))
		}
	}
	return nil
}

// modelAlias says whether a word names the agent's model, in any case,
// though a field goes by it too: Claude Code's opus, a tier's name as well
func modelAlias(a *agent.Agent, w string) bool {
	return slices.ContainsFunc(a.ModelAliases, func(m string) bool { return strings.EqualFold(m, w) })
}

// fieldAsValue refuses field g's name given as field f's value, for what
// was meant: two words that are one field's label (pigeon codex subagent
// effort), a switch, which is turned on (Claude Code's ultracode), or the
// field, which its name alone shows.
func fieldAsValue(a *agent.Agent, f, g *agent.Field, value string) error {
	for _, name := range []string{f.Key + " " + value, f.Key + "_" + value} {
		if h := a.Field(name); h != nil {
			return fmt.Errorf("%q is one field of %s's: pigeon %s %s shows it", h.Label, a.Name, a.ID, h.Key)
		}
	}
	if g.Options != nil {
		if o := g.Options(a.Values()); len(o) == 1 && o[0].Value == "on" {
			return fmt.Errorf("%q is a switch of %s's, not a value for its %s: pigeon %s %s on", value, a.Name, f.Label, a.ID, g.Key)
		}
	}
	return fmt.Errorf("%q is a field of %s's, not a value for its %s: pigeon %s %s shows it", value, a.Name, f.Label, a.ID, g.Key)
}

// showField is `pigeon <agent> <field>`: what the field is set to, as set
// prints it
func showField(a *agent.Agent, f *agent.Field) error {
	v := f.Get()
	if v == "" {
		v = muted.Render("default")
	}
	fmt.Println(bold.Render(a.Name), muted.Render(f.Label), v)
	return nil
}

// disconnect is `pigeon <agent> default` on an agent pigeon is wired into:
// the Agents page's Disconnect, which puts back what the user had before
// pigeon — Claude Code's own model, its endpoint — where a field's default
// leaves the agent as installed (__jingling on X: pigeon claude default
// took the model they had set away with pigeon's)
func disconnect(a *agent.Agent) error {
	before := a.Values()
	if err := a.Disconnect(); err != nil {
		return err
	}
	now := a.Values()
	for _, f := range a.Fields {
		if before[f.Key] == now[f.Key] {
			continue
		}
		shown := now[f.Key]
		if shown == "" {
			shown = muted.Render("default")
		}
		fmt.Println(green.Render("✓"), bold.Render(a.Name), muted.Render(f.Label), shown)
	}
	fmt.Println(muted.Render("  disconnected from pigeon, back to what it had before"))
	if a.Notice != nil {
		if n := a.Notice(); n != "" {
			fmt.Println(muted.Render("  ↻ " + n))
		}
	}
	return nil
}

func fieldForValue(a *agent.Agent, v string) *agent.Field {
	vals := a.Values()
	// the agent's suffix after a model (omp's ":max") aside: a role on the
	// same model at that level offers it as typed, and would take it. A list
	// of models no picker offers: it is for the model
	if a.SplitSuffix != nil {
		m, _, one := a.SplitSuffix(v)
		if !one {
			return nil
		}
		v = m
	}
	// a model stays with the model, even where other fields offer it too
	// (Claude Code's opus/sonnet/haiku/fable)
	for _, o := range a.Fields[0].Options(vals) {
		if o.Value == v {
			return nil
		}
	}
	for i := 1; i < len(a.Fields); i++ {
		for _, o := range a.Fields[i].Options(vals) {
			if o.Value == v {
				return &a.Fields[i]
			}
		}
	}
	return nil
}

// list prints the agents; those from dimFrom on (when not -1) are the ones
// hidden in the app, and are dimmed.
func list(agents []*agent.Agent, detectedOnly bool, dimFrom int) error {
	if len(agents) == 0 {
		return fmt.Errorf("no supported agents found on this machine")
	}
	type row struct{ name, vals, path string }
	var rows []row
	nameW, valW := 0, 0
	for i, a := range agents {
		r := row{name: a.Name, path: tilde(a.Path)}
		if !detectedOnly && !a.Detected() {
			r.name = faint.Render(a.Name)
			r.vals = faint.Render("not detected")
			r.path = ""
		} else if a.CLIMissing() {
			// its folder left by an uninstall (#2173): what it holds is
			// read by nothing
			r.name = faint.Render(a.Name)
			r.vals = faint.Render("CLI not found")
		} else {
			vals := a.Values()
			dim := dimFrom >= 0 && i >= dimFrom
			label, value := muted, lipgloss.NewStyle()
			if dim {
				label, value = faint, faint
			}
			var parts []string
			for _, f := range a.Fields {
				v := vals[f.Key]
				if v == "" && f.Quiet {
					continue
				}
				if v == "" {
					v = faint.Render("—")
				} else {
					v = value.Render(v)
				}
				if f.Label == "model" {
					parts = append(parts, v)
				} else {
					parts = append(parts, label.Render(f.Label)+" "+v)
				}
			}
			r.name = bold.Render(a.Name)
			if dim {
				r.name = faint.Render(a.Name) + " " + faint.Render("hidden")
			}
			r.vals = strings.Join(parts, label.Render("  ·  "))
			// through pigeon for account failover while not connected (#1385)
			if said := a.FailoverSaid(); said != "" {
				r.vals += label.Render("  ·  " + said)
			}
			if a.Import != nil {
				// unknown reads as not added here: the line only offers the
				// command, never says pigeon is missing
				added := false
				if a.Added != nil {
					added, _ = a.Added()
				}
				if added {
					r.vals = value.Render("pigeon added")
				} else {
					r.vals = label.Render("pigeon "+a.ID+" add") + faint.Render("  to add pigeon")
				}
			}
		}
		nameW = max(nameW, lipgloss.Width(r.name))
		valW = max(valW, lipgloss.Width(r.vals))
		rows = append(rows, r)
	}
	for _, r := range rows {
		fmt.Printf("  %s  %s  %s\n", pad(r.name, nameW), pad(r.vals, valW), faint.Render(r.path))
	}
	return nil
}

func tilde(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

func profiles(args []string) error {
	switch args[0] {
	case "profiles":
		ps, err := profile.Load()
		if err != nil {
			return err
		}
		if len(ps) == 0 {
			fmt.Println(muted.Render("no profiles yet · pigeon save <name>"))
			return nil
		}
		detected := agent.Detected()
		for _, n := range profile.Names(ps) {
			// ✓ the one the setup is now (superhtoooth on X)
			mark := " "
			if profile.InUse(ps[n], detected, nil) {
				mark = green.Render("✓")
			}
			fmt.Printf("%s %s  %s\n", mark, bold.Render(n), muted.Render(profile.LongSummary(ps[n])))
		}
		return nil
	case "save":
		if len(args) < 2 {
			return fmt.Errorf("usage: pigeon save <name>")
		}
		p, err := profile.Snapshot()
		if err != nil {
			return err
		}
		if err := profile.Save(args[1], p); err != nil {
			return err
		}
		fmt.Println(green.Render("✓"), "saved profile", bold.Render(args[1]))
		if p.Library != nil {
			fmt.Println(" ", muted.Render("with the "+p.Library.Summary()))
		}
		return nil
	case "use":
		if len(args) < 2 {
			return fmt.Errorf("usage: pigeon use <name>")
		}
		ps, err := profile.Load()
		if err != nil {
			return err
		}
		p, ok := ps[args[1]]
		if !ok {
			return fmt.Errorf("no profile named %q", args[1])
		}
		a, err := profile.Apply(p)
		if err != nil {
			return err
		}
		fmt.Println(green.Render("✓"), "applied", bold.Render(args[1]), muted.Render(fmt.Sprintf("(%d changed)", a.Changed)))
		for _, line := range profile.Report(a) {
			fmt.Println(" ", muted.Render(line))
		}
		return nil
	case "rm":
		if len(args) < 2 {
			return fmt.Errorf("usage: pigeon rm <name>")
		}
		if err := profile.Delete(args[1]); err != nil {
			return err
		}
		fmt.Println(green.Render("✓"), "deleted profile", bold.Render(args[1]))
		return nil
	}
	return nil
}

func pad(s string, w int) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

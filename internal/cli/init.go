package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lintsinghua/oops/internal/agents"
	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/platform"
	"github.com/lintsinghua/oops/internal/tui"
	"github.com/lintsinghua/oops/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func newInitCmd(opt *options) *cobra.Command {
	var yes, project, printOnly, undo, deleteData bool
	var only []string
	var pathExtra []string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Connect oops to the agents on this machine",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdInit(opt, yes, project, printOnly, undo, deleteData, only, pathExtra)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Connect without asking")
	cmd.Flags().BoolVar(&project, "project", false, "Write project config instead of user config")
	cmd.Flags().BoolVar(&printOnly, "print", false, "Print the config fragment without writing it")
	cmd.Flags().BoolVar(&undo, "undo", false, "Remove oops hooks")
	cmd.Flags().BoolVar(&deleteData, "delete-data", false, "With --undo, also delete snapshot data")
	cmd.Flags().StringArrayVar(&only, "agent", nil, "Only this agent (repeatable)")
	cmd.Flags().StringArrayVar(&pathExtra, "path", nil, "Extra directories to search for agent binaries")
	_ = cmd.Flags().MarkHidden("path")
	cmd.Example = "  oops init -y\n  oops init --undo"
	return cmd
}

type foundAgent struct {
	agent agents.Agent
	inst  agents.Installation
}

func cmdInit(opt *options, yes, project, printOnly, undo, deleteData bool, only, pathExtra []string) error {
	user := opt.userHome
	if user == "" {
		var err error
		user, err = os.UserHomeDir()
		if err != nil {
			return err
		}
	}
	projectDir := ""
	if project || undo {
		dir := opt.dir
		if dir == "" {
			var err error
			dir, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		root, err := platform.FindRoot(dir)
		if err != nil {
			return err
		}
		projectDir = root
	}
	list := selectAgents(only)
	if len(only) > 0 && len(list) == 0 {
		return fail(2, "Unknown agent. oops knows Claude Code, Codex, Gemini CLI, and Cursor CLI.")
	}
	var rows []foundAgent
	for _, a := range list {
		rows = append(rows, foundAgent{agent: a, inst: agents.Detect(a, user, projectDir, project, pathExtra)})
	}
	exe, err := os.Executable()
	if err != nil || exe == "" {
		exe = "oops"
	}
	if undo {
		return cmdInitUndo(opt, rows, yes, project, deleteData, user, projectDir)
	}
	if printOnly {
		return cmdInitPrint(opt, rows, exe)
	}
	return cmdInitWrite(opt, rows, exe, yes, project, user, projectDir)
}

func selectAgents(only []string) []agents.Agent {
	if len(only) == 0 {
		return agents.Agents()
	}
	var out []agents.Agent
	for _, name := range only {
		if a, ok := agents.ByName(name); ok {
			out = append(out, a)
		}
		for _, a := range agents.Agents() {
			if strings.EqualFold(a.Name, name) || strings.EqualFold(shortAgent(a.Name), name) {
				dup := false
				for _, e := range out {
					if e.Name == a.Name {
						dup = true
					}
				}
				if !dup {
					out = append(out, a)
				}
			}
		}
	}
	return out
}

// needsInstall is a missing agent, or one whose end hook still runs in the
// background. Re-running init rewrites those async entries.
func needsInstall(inst agents.Installation) bool {
	return inst.Found && !inst.Invalid && (!inst.Connected || inst.Stale)
}

func shortAgent(name string) string {
	switch name {
	case "Claude Code":
		return "claude"
	case "Gemini CLI":
		return "gemini"
	case "Cursor CLI":
		return "cursor"
	default:
		return strings.ToLower(name)
	}
}

func cmdInitPrint(opt *options, rows []foundAgent, exe string) error {
	for _, row := range rows {
		if !row.inst.Found || row.inst.Invalid {
			continue
		}
		ch, changed, err := row.agent.Install(exe, row.inst.ConfigPath, false)
		if err != nil {
			fmt.Fprintf(opt.stderr, "! %s  %s\n", row.agent.Name, err.Error())
			continue
		}
		if !changed && row.inst.Connected {
			fmt.Fprintf(opt.stdout, "# %s already connected\n", row.agent.Name)
			continue
		}
		fmt.Fprintf(opt.stdout, "# %s  %s\n%s\n", row.agent.Name, row.inst.ConfigPath, string(ch.After))
	}
	return nil
}

func cmdInitWrite(opt *options, rows []foundAgent, exe string, yes, project bool, user, projectDir string) error {
	found := 0
	for _, row := range rows {
		if row.inst.Found {
			found++
		}
	}
	st, sym := stylesOf(opt)
	if !opt.json && !opt.quiet {
		fmt.Fprintln(opt.stdout)
		fmt.Fprintln(opt.stdout, "  oops keeps a snapshot before every agent turn, so you can undo it.")
		fmt.Fprintln(opt.stdout)
		if found == 0 {
			fmt.Fprintln(opt.stdout, "  No agents found. oops still works with any tool:  oops run -- <command>")
			fmt.Fprintln(opt.stdout)
			for _, row := range rows {
				fmt.Fprintf(opt.stdout, "    %s  %s\n", row.agent.Name, row.agent.Docs)
			}
			return nil
		}
		fmt.Fprintf(opt.stdout, "  Found %s\n", ui.Plural(found, "agent", "agents"))
		for _, row := range rows {
			mark := sym.Off
			ver := "not found"
			path := ""
			if row.inst.Found {
				mark = sym.On
				ver = row.inst.Version
				path = "   " + displayHome(row.inst.ConfigPath, user)
			}
			line := fmt.Sprintf("    %s %s   %s%s", mark, ui.Pad(row.agent.Name, 12), ui.Pad(ver, 10), path)
			if row.inst.Found {
				fmt.Fprintln(opt.stdout, st.Success.Render(line))
			} else {
				fmt.Fprintln(opt.stdout, st.Muted.Render(line))
			}
		}
		fmt.Fprintln(opt.stdout)
	}
	pending := 0
	for _, row := range rows {
		if needsInstall(row.inst) {
			pending++
		}
	}
	if pending == 0 && found > 0 {
		if opt.json {
			return writeInitJSON(opt, rows, nil)
		}
		for _, row := range rows {
			if !row.inst.Found {
				continue
			}
			if row.inst.Invalid {
				fmt.Fprintf(opt.stdout, "  %s %s  %s is not valid JSON. Skipped — fix it and run oops init again.\n", sym.Warn, ui.Pad(row.agent.Name, 12), displayHome(row.inst.ConfigPath, user))
				continue
			}
			note := "already connected"
			if row.agent.TrustNote != "" {
				note += " · " + row.agent.TrustNote
			}
			fmt.Fprintf(opt.stdout, "  %s %s  %s\n", sym.OK, ui.Pad(row.agent.Name, 12), note)
		}
		fmt.Fprintln(opt.stdout, "  Nothing to do.")
		return nil
	}
	chosen := map[string]bool{}
	for _, row := range rows {
		if needsInstall(row.inst) {
			chosen[row.agent.Name] = true
		}
	}
	if !yes {
		if !canAsk(opt) {
			fmt.Fprintln(opt.stdout, "  Run oops init -y to connect.")
			return fail(2, "")
		}
		fmt.Fprintf(opt.stderr, "  oops will add its hooks to these %s. Backups are saved next to them.\n", ui.Plural(pending, "file", "files"))
		fmt.Fprintf(opt.stderr, "  Snapshots stay on this machine in %s. Nothing is uploaded.\n\n", dataHomeLine(opt))
		fmt.Fprintf(opt.stderr, "  Connect all %d? [Y/n/e] ", pending)
		line, err := readLine(opt.stdin)
		if err != nil {
			return err
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "", "y", "yes":
		case "n", "no":
			fmt.Fprintln(opt.stdout, "Nothing changed.")
			return errStop
		case "e":
			var labels []string
			var on []bool
			var names []string
			for _, row := range rows {
				if !row.inst.Found || row.inst.Invalid {
					continue
				}
				labels = append(labels, row.agent.Name+"   "+row.inst.Version)
				names = append(names, row.agent.Name)
				on = append(on, needsInstall(row.inst))
			}
			picked, err := runPick(opt, labels, on)
			if err != nil {
				return err
			}
			chosen = map[string]bool{}
			for i, name := range names {
				if i < len(picked) && picked[i] {
					chosen[name] = true
				}
			}
		default:
			fmt.Fprintln(opt.stdout, "Nothing changed.")
			return errStop
		}
	}
	connected := 0
	var problems int
	for _, row := range rows {
		if !chosen[row.agent.Name] {
			continue
		}
		if row.inst.Invalid {
			fmt.Fprintf(opt.stdout, "  %s %s  %s is not valid JSON. Skipped — fix it and run oops init again.\n", sym.Warn, ui.Pad(row.agent.Name, 12), displayHome(row.inst.ConfigPath, user))
			problems++
			continue
		}
		_, changed, err := row.agent.Install(exe, row.inst.ConfigPath, true)
		if err != nil {
			fmt.Fprintf(opt.stdout, "  %s %s  %s\n", sym.Warn, row.agent.Name, err.Error())
			problems++
			continue
		}
		if row.agent.Name == "Claude Code" {
			if err := writeClaudeCommand(user, projectDir, project); err != nil {
				fmt.Fprintf(opt.stderr, "  %s could not install /oops: %s\n", sym.Warn, err.Error())
			}
		}
		note := "connected"
		if !changed && row.inst.Connected {
			note = "already connected"
		}
		if row.agent.TrustNote != "" {
			note += " · " + row.agent.TrustNote
		} else if row.agent.SlashNote != "" {
			note += " · " + row.agent.SlashNote
		}
		if !opt.json {
			fmt.Fprintf(opt.stdout, "  %s %s  %s\n", sym.OK, ui.Pad(row.agent.Name, 12), note)
		}
		connected++
	}
	if opt.json {
		return writeInitJSON(opt, rows, chosen)
	}
	if connected == 0 {
		return fail(1, "")
	}
	fmt.Fprintln(opt.stdout)
	fmt.Fprintln(opt.stdout, "  You're protected.")
	fmt.Fprintln(opt.stdout, "  Undo this setup any time: oops init --undo")
	oops := "oops"
	if opt.colorOn() {
		oops = st.Accent.Render("oops")
	}
	fmt.Fprintf(opt.stdout, "  When an agent breaks something, just type: %s\n", oops)
	_ = problems
	return nil
}

func cmdInitUndo(opt *options, rows []foundAgent, yes, project, deleteData bool, user, projectDir string) error {
	var present []foundAgent
	for _, row := range rows {
		if row.inst.Connected {
			present = append(present, row)
		}
	}
	if len(present) == 0 {
		if opt.json {
			fmt.Fprintln(opt.stdout, `{"schema_version":1,"removed":[]}`)
			return nil
		}
		fmt.Fprintln(opt.stdout, "  No oops hooks to remove.")
		return nil
	}
	if !yes {
		if !canAsk(opt) {
			return fail(2, "oops init --undo needs confirmation. Re-run with -y to proceed.")
		}
		fmt.Fprintf(opt.stderr, "\n  Remove oops hooks from %s?\n", ui.Plural(len(present), "agent", "agents"))
		for _, row := range present {
			fmt.Fprintf(opt.stderr, "    %s  %s\n", ui.Pad(row.agent.Name, 12), displayHome(row.inst.ConfigPath, user))
		}
		fmt.Fprintln(opt.stderr, "  Your other settings stay untouched.")
		fmt.Fprintln(opt.stderr)
		fmt.Fprint(opt.stderr, "  Remove? [y/N] ")
		ok, err := ask(opt.stdin, false)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(opt.stdout, "Nothing changed.")
			return errStop
		}
	}
	n := 0
	sym := ui.Glyphs(ui.ASCIIMode())
	for _, row := range present {
		if _, changed, err := row.agent.Uninstall(row.inst.ConfigPath); err != nil {
			return err
		} else if changed {
			n++
		}
		if row.agent.Name == "Claude Code" {
			_ = removeClaudeCommand(user, projectDir, project)
		}
	}
	if opt.json {
		fmt.Fprintf(opt.stdout, "{\"schema_version\":1,\"removed\":%d}\n", n)
		return nil
	}
	fmt.Fprintf(opt.stdout, "  %s Removed from %s.\n", sym.OK, ui.Plural(n, "agent", "agents"))
	fmt.Fprintln(opt.stdout)
	if deleteData {
		if err := deleteSnapshotData(opt, yes); err != nil {
			return err
		}
		return nil
	}
	fmt.Fprintf(opt.stdout, "  Snapshots kept (in %s).\n", dataHomeLine(opt))
	fmt.Fprintln(opt.stdout, "  Delete them too: oops gc --all")
	return nil
}

func deleteSnapshotData(opt *options, yes bool) error {
	if !yes {
		if !canAsk(opt) {
			return fail(2, "oops init --undo --delete-data needs confirmation. Re-run with -y to proceed.")
		}
		fmt.Fprint(opt.stderr, "  Delete all oops snapshots too? [y/N] ")
		ok, err := ask(opt.stdin, false)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(opt.stdout, "  Snapshots kept.")
			return nil
		}
	}
	e, err := openEngine(opt)
	if err != nil {
		return err
	}
	defer e.Close()
	list, err := e.DB.Workspaces()
	if err != nil {
		return err
	}
	for _, w := range list {
		if _, err := e.GC(engine.Workspace{ID: w.ID, Path: w.Path}, false, true); err != nil {
			return err
		}
	}
	fmt.Fprintln(opt.stdout, "  Snapshots deleted.")
	return nil
}

func writeClaudeCommand(user, project string, projectLevel bool) error {
	path := agents.ClaudeCommandPath(user, project, projectLevel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(agents.ClaudeCommandBody), 0o644)
}

func removeClaudeCommand(user, project string, projectLevel bool) error {
	path := agents.ClaudeCommandPath(user, project, projectLevel)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if !strings.Contains(string(b), "<!-- oops -->") {
		return nil
	}
	return os.Remove(path)
}

func writeInitJSON(opt *options, rows []foundAgent, chosen map[string]bool) error {
	type row struct {
		Name      string `json:"name"`
		Found     bool   `json:"found"`
		Version   string `json:"version,omitempty"`
		Config    string `json:"config"`
		Connected bool   `json:"connected"`
		Invalid   bool   `json:"invalid,omitempty"`
	}
	out := struct {
		SchemaVersion int   `json:"schema_version"`
		Agents        []row `json:"agents"`
	}{SchemaVersion: 1}
	for _, r := range rows {
		if chosen != nil && !chosen[r.agent.Name] && !r.inst.Connected {
			continue
		}
		out.Agents = append(out.Agents, row{
			Name: r.agent.Name, Found: r.inst.Found, Version: r.inst.Version,
			Config: r.inst.ConfigPath, Connected: r.inst.Connected || (chosen != nil && chosen[r.agent.Name]),
			Invalid: r.inst.Invalid,
		})
	}
	enc := json.NewEncoder(opt.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func runPick(opt *options, labels []string, on []bool) ([]bool, error) {
	m := tui.NewPick("Choose agents to connect   (space to toggle, enter to confirm)", labels, on)
	prog := tea.NewProgram(m, tea.WithInput(opt.stdin), tea.WithOutput(opt.stdout))
	final, err := prog.Run()
	if err != nil {
		return nil, err
	}
	picked, ok := final.(tui.Pick)
	if !ok || picked.Cancelled() {
		fmt.Fprintln(opt.stdout, "Nothing changed.")
		return nil, errStop
	}
	return picked.On, nil
}

func displayHome(path, user string) string {
	if user != "" && strings.HasPrefix(path, user) {
		return "~" + strings.TrimPrefix(path, user)
	}
	return path
}

func dataHomeLine(opt *options) string {
	if opt.home != "" {
		return opt.home
	}
	if v := os.Getenv("OOPS_HOME"); v != "" {
		return v
	}
	return "~/.local/share/oops"
}

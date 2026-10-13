package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lintsinghua/oops/internal/agents"
	"github.com/lintsinghua/oops/internal/config"
	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/platform"
	"github.com/lintsinghua/oops/internal/snapshot"
	"github.com/lintsinghua/oops/internal/ui"
	"github.com/lintsinghua/oops/internal/version"
)

func newDoctorCmd(opt *options) *cobra.Command {
	var pathExtra []string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check that everything works",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdDoctor(opt, pathExtra)
		},
	}
	cmd.Flags().StringArrayVar(&pathExtra, "path", nil, "Extra directories to search for agent binaries")
	_ = cmd.Flags().MarkHidden("path")
	return cmd
}

type check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

func cmdDoctor(opt *options, pathExtra []string) error {
	e, err := openEngine(opt)
	if err != nil {
		return err
	}
	defer e.Close()
	var checks []check
	info := version.Current()
	exe, _ := os.Executable()
	checks = append(checks, check{Name: "binary", Status: "ok", Detail: fmt.Sprintf("oops %s (%s/%s) at %s", info.Version, runtime.GOOS, runtime.GOARCH, exe)})

	user := e.UserHome
	if user == "" {
		user, _ = os.UserHomeDir()
	}
	for _, a := range agents.Agents() {
		inst := agents.Detect(a, user, "", false, pathExtra)
		if !inst.Found && !inst.Connected {
			continue
		}
		switch {
		case inst.Invalid:
			checks = append(checks, check{Name: a.Name, Status: "warn", Detail: inst.ConfigPath + " is not valid JSON", Fix: "fix the file and run oops init"})
		case inst.Connected && a.TrustNote != "":
			checks = append(checks, check{Name: a.Name, Status: "warn", Detail: "hooks installed but not trusted yet — " + a.TrustNote, Fix: "open the agent and run /hooks"})
		case inst.Connected:
			checks = append(checks, check{Name: a.Name, Status: "ok", Detail: "hooks installed"})
		default:
			checks = append(checks, check{Name: a.Name, Status: "warn", Detail: "installed, hooks not connected", Fix: "oops init"})
		}
	}

	st, err := os.Stat(e.Home)
	if err != nil {
		checks = append(checks, check{Name: "storage", Status: "fail", Detail: e.Home + " is not usable", Fix: "mkdir -p " + e.Home})
	} else if st.Mode().Perm()&0o077 != 0 {
		checks = append(checks, check{Name: "storage", Status: "warn", Detail: e.Home + " is readable by other users", Fix: "chmod 700 " + e.Home})
	}

	var ws engine.Workspace
	ws, wsErr := loadWS(opt, e)
	if wsErr != nil {
		checks = append(checks, check{Name: "workspace", Status: "warn", Detail: wsErr.Error(), Fix: "run oops inside a project folder"})
	} else {
		files, bytes, dirs := walkCount(e, ws.Path)
		detail := fmt.Sprintf("%s · %s · %s protected", platform.DisplayPath(ws.Path, user), comma(files)+" files", ui.Size(bytes))
		status := "ok"
		fix := ""
		if files > 200000 {
			status = "warn"
			fix = "add bulky folders to .oopsignore"
			detail += " — over 200k files"
		}
		checks = append(checks, check{Name: "workspace", Status: status, Detail: detail, Fix: fix})
		if snap, err := e.DB.LatestSnapshot(ws.ID); err == nil {
			if len(snap.Skipped) > 0 {
				var parts []string
				for _, sk := range snap.Skipped {
					parts = append(parts, fmt.Sprintf("%s (%s)", sk.Path, ui.Size(sk.Size)))
				}
				checks = append(checks, check{
					Name: "large files", Status: "warn",
					Detail: ui.Plural(len(snap.Skipped), "file too large to protect", "files too large to protect") + ": " + strings.Join(parts, ", "),
					Fix:    "add them to .oopsignore, or raise snapshot.max_file_size",
				})
			}
			when := ui.RelTime(e.Now, snap.CreatedAt)
			extra := ""
			if snap.Partial {
				extra = " (partial)"
			}
			checks = append(checks, check{Name: "last snapshot", Status: "ok", Detail: when + extra})
		}
		n, _ := e.DB.CountTurns(ws.ID)
		stored, _ := e.Store.Size()
		storage := check{Name: "storage", Status: "ok", Detail: fmt.Sprintf("%s of %s · %s", ui.Size(stored), e.Config.Retention.WorkspaceMax, ui.Plural(n, "turn", "turns"))}
		if maxWS, err := config.ParseSize(e.Config.Retention.WorkspaceMax); err == nil && maxWS > 0 && stored > maxWS {
			storage.Status = "warn"
			storage.Fix = "oops gc"
			storage.Detail += " · over the workspace cap"
		}
		checks = append(checks, storage)
		if runtime.GOOS == "linux" {
			if watches, err := inotifyMax(); err == nil && dirs > watches {
				checks = append(checks, check{
					Name: "inotify", Status: "warn",
					Detail: fmt.Sprintf("%d directories and only %d inotify watches", dirs, watches),
					Fix:    "raise fs.inotify.max_user_watches or add large folders to .oopsignore",
				})
			}
		}
	}

	warnings := 0
	fails := 0
	for _, c := range checks {
		if c.Status == "warn" {
			warnings++
		}
		if c.Status == "fail" {
			fails++
		}
	}
	if opt.json {
		enc := json.NewEncoder(opt.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			SchemaVersion int     `json:"schema_version"`
			Checks        []check `json:"checks"`
			Warnings      int     `json:"warnings"`
		}{1, checks, warnings})
	}
	sym := ui.Glyphs(ui.ASCIIMode())
	for _, c := range checks {
		mark := sym.OK
		switch c.Status {
		case "warn":
			mark = sym.Warn
		case "fail":
			mark = sym.Fail
		}
		line := fmt.Sprintf("  %s %-14s %s", mark, c.Name, c.Detail)
		fmt.Fprintln(opt.stdout, line)
		if c.Fix != "" && c.Status != "ok" {
			fmt.Fprintf(opt.stdout, "                   → %s\n", c.Fix)
		}
	}
	fmt.Fprintln(opt.stdout)
	fmt.Fprintf(opt.stdout, "  %s.\n", ui.Plural(warnings, "warning", "warnings"))
	if fails > 0 {
		return fail(1, "")
	}
	return nil
}

func walkCount(e *engine.Engine, root string) (files int, bytes int64, dirs int) {
	rules, err := snapshot.LoadRules(root, e.Config.Snapshot.Protect, e.Config.Snapshot.Exclude, e.Config.Snapshot.RespectGitignore)
	if err != nil {
		rules, _ = snapshot.LoadRules(root, nil, nil, true)
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rules != nil && rules.PruneDir(rel) {
				return filepath.SkipDir
			}
			dirs++
			return nil
		}
		if rules != nil && !rules.Include(rel, false) {
			return nil
		}
		files++
		if info, err := d.Info(); err == nil {
			bytes += info.Size()
		}
		return nil
	})
	return files, bytes, dirs
}

func inotifyMax() (int, error) {
	b, err := os.ReadFile("/proc/sys/fs/inotify/max_user_watches")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

func comma(n int) string {
	s := strconv.Itoa(n)
	var out []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

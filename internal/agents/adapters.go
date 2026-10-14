package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Agent is one supported coding agent.
type Agent struct {
	Name      string
	Binary    []string
	Docs      string
	Config    func(home, project string, projectLevel bool) string
	Install   func(exe, path string, write bool) (Change, bool, error)
	Uninstall func(path string) (Change, bool, error)
	SlashNote string
	TrustNote string
	// EndEvents are the hooks that close a turn. They stay synchronous so the
	// agent does not exit before the turn is marked ended.
	EndEvents []string
}

// Agents is the v1 adapter list, in display order.
func Agents() []Agent {
	return []Agent{Claude(), Codex(), Gemini(), Cursor()}
}

// ByName finds an adapter.
func ByName(name string) (Agent, bool) {
	for _, a := range Agents() {
		if a.Name == name || strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return Agent{}, false
}

// Claude returns the Claude Code adapter.
// Docs: https://code.claude.com/docs/en/hooks
func Claude() Agent {
	return Agent{
		Name:   "Claude Code",
		Binary: []string{"claude"},
		Docs:   "https://code.claude.com/docs/en/hooks",
		Config: func(home, project string, projectLevel bool) string {
			if projectLevel {
				return filepath.Join(project, ".claude", "settings.json")
			}
			return filepath.Join(home, ".claude", "settings.json")
		},
		SlashNote: "/oops ready",
		EndEvents: claudeEndEvents,
		Install: func(exe, path string, write bool) (Change, bool, error) {
			doc, prev, err := ReadJSON(path)
			if err != nil {
				return Change{}, false, err
			}
			groups := map[string][]map[string]any{
				"SessionStart":        {commandAsync(exe, []string{"hook", "claude", "session-start"})},
				"UserPromptSubmit":    {commandTimeout(exe, []string{"hook", "claude", "turn-start"}, 15)},
				"UserPromptExpansion": {commandMatch(exe, []string{"hook", "claude", "slash"}, "oops")},
				"Stop":                {commandSync(exe, []string{"hook", "claude", "turn-end"})},
				"StopFailure":         {commandSync(exe, []string{"hook", "claude", "turn-end"})},
				"SubagentStop":        {commandSync(exe, []string{"hook", "claude", "subagent-stop"})},
			}
			synced := ClearAsync(doc, claudeEndEvents)
			if !EnsureHooks(doc, groups) && !synced && Connected(doc) {
				return Change{Path: path, Before: prev, After: prev}, false, nil
			}
			return finishWrite(path, doc, prev, write)
		},
		Uninstall: uninstallFile,
	}
}

// Codex returns the Codex adapter.
// Docs: https://developers.openai.com/codex/hooks
func Codex() Agent {
	return Agent{
		Name:      "Codex",
		Binary:    []string{"codex"},
		Docs:      "https://developers.openai.com/codex/hooks",
		TrustNote: "open Codex and run /hooks once to trust oops",
		EndEvents: codexEndEvents,
		Config: func(home, project string, projectLevel bool) string {
			if projectLevel {
				return filepath.Join(project, ".codex", "hooks.json")
			}
			return filepath.Join(home, ".codex", "hooks.json")
		},
		Install: func(exe, path string, write bool) (Change, bool, error) {
			doc, prev, err := ReadJSON(path)
			if err != nil {
				return Change{}, false, err
			}
			if doc["description"] == nil {
				doc["description"] = "oops — undo for coding agents (managed by `oops init`)"
			}
			cmd := exe + " hook codex"
			groups := map[string][]map[string]any{
				"UserPromptSubmit": {shellHook(cmd+" turn-start", 15, "")},
				"Stop":             {shellHook(cmd+" turn-end", 15, "")},
				"Interrupt":        {shellHook(cmd+" turn-end", 3, "")},
			}
			synced := ClearAsync(doc, codexEndEvents)
			if !EnsureHooks(doc, groups) && !synced && Connected(doc) {
				return Change{Path: path, Before: prev, After: prev}, false, nil
			}
			return finishWrite(path, doc, prev, write)
		},
		Uninstall: uninstallFile,
	}
}

// Gemini returns the Gemini CLI adapter.
// Docs: https://geminicli.com/docs/hooks/reference/
func Gemini() Agent {
	return Agent{
		Name:      "Gemini CLI",
		Binary:    []string{"gemini"},
		Docs:      "https://geminicli.com/docs/hooks/reference/",
		EndEvents: geminiEndEvents,
		Config: func(home, project string, projectLevel bool) string {
			if projectLevel {
				return filepath.Join(project, ".gemini", "settings.json")
			}
			return filepath.Join(home, ".gemini", "settings.json")
		},
		Install: func(exe, path string, write bool) (Change, bool, error) {
			doc, prev, err := ReadJSON(path)
			if err != nil {
				return Change{}, false, err
			}
			cmd := exe + " hook gemini"
			groups := map[string][]map[string]any{
				"BeforeAgent": {namedHook("oops-turn-start", cmd+" turn-start", 15000)},
				"AfterAgent":  {namedHook("oops-turn-end", cmd+" turn-end", 15000)},
			}
			synced := ClearAsync(doc, geminiEndEvents)
			if !EnsureHooks(doc, groups) && !synced && Connected(doc) {
				return Change{Path: path, Before: prev, After: prev}, false, nil
			}
			return finishWrite(path, doc, prev, write)
		},
		Uninstall: uninstallFile,
	}
}

// Cursor returns the Cursor CLI adapter.
// Docs: https://cursor.com/docs/hooks
func Cursor() Agent {
	return Agent{
		Name:      "Cursor CLI",
		Binary:    []string{"cursor-agent", "agent"},
		Docs:      "https://cursor.com/docs/hooks",
		SlashNote: "shell escape: !oops undo",
		EndEvents: cursorEndEvents,
		Config: func(home, project string, projectLevel bool) string {
			if projectLevel {
				return filepath.Join(project, ".cursor", "hooks.json")
			}
			return filepath.Join(home, ".cursor", "hooks.json")
		},
		Install: func(exe, path string, write bool) (Change, bool, error) {
			doc, prev, err := ReadJSON(path)
			if err != nil {
				return Change{}, false, err
			}
			if doc["version"] == nil {
				doc["version"] = 1
			}
			cmd := exe + " hook cursor"
			groups := map[string][]map[string]any{
				"beforeSubmitPrompt":   {FlatCommand(cmd + " turn-start")},
				"stop":                 {FlatCommand(cmd + " turn-end")},
				"beforeShellExecution": {FlatCommand(cmd + " step")},
				"afterFileEdit":        {FlatCommand(cmd + " step")},
			}
			synced := ClearAsync(doc, cursorEndEvents)
			if !ensureCursor(doc, groups) && !synced && Connected(doc) {
				return Change{Path: path, Before: prev, After: prev}, false, nil
			}
			return finishWrite(path, doc, prev, write)
		},
		Uninstall: uninstallFile,
	}
}

func ensureCursor(doc map[string]any, groups map[string][]map[string]any) bool {
	hooks, _ := doc["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		doc["hooks"] = hooks
	}
	changed := false
	for event, want := range groups {
		if eventConnected(hooks[event]) {
			continue
		}
		list, _ := hooks[event].([]any)
		for _, g := range want {
			list = append(list, g)
		}
		hooks[event] = list
		changed = true
	}
	return changed
}

var (
	claudeEndEvents = []string{"Stop", "StopFailure", "SubagentStop"}
	codexEndEvents  = []string{"Stop", "Interrupt"}
	geminiEndEvents = []string{"AfterAgent"}
	cursorEndEvents = []string{"stop"}
)

func commandTimeout(exe string, args []string, timeout int) map[string]any {
	return CommandGroup(exe, args, timeout, false, "", "")
}

func commandAsync(exe string, args []string) map[string]any {
	return CommandGroup(exe, args, 0, true, "", "")
}

func commandSync(exe string, args []string) map[string]any {
	return CommandGroup(exe, args, 0, false, "", "")
}

func commandMatch(exe string, args []string, matcher string) map[string]any {
	return CommandGroup(exe, args, 0, false, matcher, "")
}

func shellHook(command string, timeout int, name string) map[string]any {
	return CommandGroup(command, nil, timeout, false, "", name)
}

func namedHook(name, command string, timeout int) map[string]any {
	return CommandGroup(command, nil, timeout, false, "", name)
}

func uninstallFile(path string) (Change, bool, error) {
	doc, prev, err := ReadJSON(path)
	if err != nil {
		return Change{}, false, err
	}
	backups := listBackups(path)
	if len(prev) == 0 && len(backups) == 0 {
		return Change{Path: path}, false, nil
	}
	// Init saved the pre-edit bytes. Put those back so a user's hooks file
	// returns exactly, including key order and whitespace, then delete the copies.
	if orig, ok := oldestBackup(backups); ok {
		if err := writeBytesAtomic(path, orig); err != nil {
			return Change{}, false, err
		}
		removeBackups(backups)
		return Change{Path: path, Before: prev, After: orig}, !bytesEqual(prev, orig), nil
	}
	next, changed := RemoveOops(doc)
	obj, _ := next.(map[string]any)
	if obj == nil {
		obj = map[string]any{}
	}
	if !changed {
		removeBackups(backups)
		return Change{Path: path, Before: prev, After: prev}, false, nil
	}
	if oopsOnly(obj) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return Change{}, false, err
		}
		removeBackups(backups)
		removeEmptyDirs(filepath.Dir(path), 1)
		return Change{Path: path, Before: prev}, true, nil
	}
	ch, err := WriteJSON(path, obj, prev)
	removeBackups(append(backups, listBackups(path)...))
	return ch, true, err
}

func listBackups(path string) []string {
	dir := filepath.Dir(path)
	prefix := filepath.Base(path) + ".oops-backup-"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), prefix) {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

func oldestBackup(paths []string) ([]byte, bool) {
	if len(paths) == 0 {
		return nil, false
	}
	sort.Slice(paths, func(i, j int) bool {
		si, ei := os.Stat(paths[i])
		sj, ej := os.Stat(paths[j])
		if ei != nil || ej != nil {
			return paths[i] < paths[j]
		}
		if !si.ModTime().Equal(sj.ModTime()) {
			return si.ModTime().Before(sj.ModTime())
		}
		return paths[i] < paths[j]
	})
	b, err := os.ReadFile(paths[0])
	if err != nil {
		return nil, false
	}
	return b, true
}

// removeEmptyDirs deletes dir, then its parents, while each one is empty.
// levels caps the walk so a home directory is left in place.
func removeEmptyDirs(dir string, levels int) {
	for i := 0; i < levels; i++ {
		if dir == "" || dir == "." || dir == string(filepath.Separator) {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		next := filepath.Dir(dir)
		if next == dir {
			return
		}
		dir = next
	}
}

func removeBackups(paths []string) {
	seen := map[string]struct{}{}
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		_ = os.Remove(p)
	}
}

func writeBytesAtomic(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp := path + ".oops-tmp"
	if err := os.WriteFile(tmp, raw, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// oopsOnly reports a document that contains nothing but keys oops itself added.
func oopsOnly(doc map[string]any) bool {
	if Connected(doc) {
		return false
	}
	for k, v := range doc {
		switch k {
		case "hooks":
			if !dropEmpty(v) {
				return false
			}
		case "version":
		case "description":
			s, _ := v.(string)
			if !strings.Contains(strings.ToLower(s), "oops") {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func finishWrite(path string, doc map[string]any, prev []byte, write bool) (Change, bool, error) {
	if !write {
		raw, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return Change{}, false, err
		}
		raw = append(raw, '\n')
		return Change{Path: path, Before: prev, After: raw}, !bytesEqual(prev, raw), nil
	}
	ch, err := WriteJSON(path, doc, prev)
	if err != nil {
		return ch, false, err
	}
	return ch, !bytesEqual(ch.Before, ch.After), nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var verRe = regexp.MustCompile(`v?\d+\.\d+(?:\.\d+)?(?:[-+][0-9A-Za-z.-]+)?`)

// Detect finds binaries and whether hooks are already installed.
func Detect(a Agent, home, project string, projectLevel bool, pathExtra []string) Installation {
	inst := Installation{Name: a.Name, ConfigPath: a.Config(home, project, projectLevel)}
	bin, ok := look(a.Binary, pathExtra)
	inst.Found = ok
	if ok {
		inst.Version = versionOf(bin)
		if v, ok := normalizeSemver(inst.Version); ok {
			inst.Version = v
		} else {
			inst.Version = "unknown"
		}
	}
	doc, _, err := ReadJSON(inst.ConfigPath)
	if err != nil {
		inst.Invalid = true
		inst.InvalidMsg = err.Error()
	} else {
		inst.Connected = Connected(doc)
		inst.Stale = HasAsyncEndHook(doc, a.EndEvents)
	}
	return inst
}

func look(names, extra []string) (string, bool) {
	path := os.Getenv("PATH")
	if len(extra) > 0 {
		path = strings.Join(extra, string(os.PathListSeparator)) + string(os.PathListSeparator) + path
	}
	for _, name := range names {
		for _, dir := range filepath.SplitList(path) {
			candidate := filepath.Join(dir, name)
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				return candidate, true
			}
			if os.PathSeparator == '\\' {
				for _, ext := range []string{".exe", ".cmd", ".bat"} {
					p := candidate + ext
					if st, err := os.Stat(p); err == nil && !st.IsDir() {
						return p, true
					}
				}
			}
		}
	}
	return "", false
}

// versionOf reads a version without surprising side effects.
// Gemini CLI rewrites ~/.gemini/settings.json on any launch, so its version
// comes from the installed @google/gemini-cli package.json and the binary is
// never executed. Claude Code, Codex, and Cursor do not rewrite the hook
// config oops manages when asked for --version, so they may still be executed.
// A value that is not itself a semver is discarded by Detect.
func versionOf(bin string) string {
	if isGeminiBin(bin) {
		return geminiVersion(bin)
	}
	if v := findVersion(commandStdout(bin, "--version")); v != "" {
		return v
	}
	return findVersion(commandStdout(bin, "version"))
}

func isGeminiBin(bin string) bool {
	base := strings.ToLower(filepath.Base(bin))
	for _, ext := range []string{".exe", ".cmd", ".bat", ".ps1"} {
		base = strings.TrimSuffix(base, ext)
	}
	return base == "gemini"
}

func geminiVersion(bin string) string {
	seen := map[string]bool{}
	var roots []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		roots = append(roots, p)
	}
	add(bin)
	if resolved, err := filepath.EvalSymlinks(bin); err == nil {
		add(resolved)
	}
	if b, err := os.ReadFile(bin); err == nil && looksLikeScript(b) {
		for _, p := range quotedPaths(string(b)) {
			add(p)
		}
	}
	for _, root := range roots {
		if v := geminiVersionNear(root); v != "" {
			return v
		}
	}
	return ""
}

func looksLikeScript(b []byte) bool {
	if len(b) == 0 || len(b) > 1<<16 || bytes.IndexByte(b, 0) >= 0 {
		return false
	}
	return bytes.HasPrefix(b, []byte("#!")) || bytes.HasPrefix(b, []byte("@echo"))
}

func quotedPaths(body string) []string {
	var out []string
	for _, q := range []string{`"`, `'`} {
		rest := body
		for {
			i := strings.Index(rest, q)
			if i < 0 {
				break
			}
			rest = rest[i+len(q):]
			j := strings.Index(rest, q)
			if j < 0 {
				break
			}
			s := rest[:j]
			rest = rest[j+len(q):]
			if strings.Contains(s, "/") || strings.Contains(s, `\`) {
				out = append(out, s)
			}
		}
	}
	return out
}

func geminiVersionNear(path string) string {
	dir := path
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		dir = filepath.Dir(path)
	}
	rels := []string{
		filepath.Join(dir, "..", "lib", "node_modules", "@google", "gemini-cli", "package.json"),
		filepath.Join(dir, "..", "node_modules", "@google", "gemini-cli", "package.json"),
		filepath.Join(dir, "node_modules", "@google", "gemini-cli", "package.json"),
	}
	for _, p := range rels {
		if v := readGeminiPackage(p); v != "" {
			return v
		}
	}
	cur := dir
	for i := 0; i < 6; i++ {
		if v := readGeminiPackage(filepath.Join(cur, "package.json")); v != "" {
			return v
		}
		if v := readGeminiPackage(filepath.Join(cur, "node_modules", "@google", "gemini-cli", "package.json")); v != "" {
			return v
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return ""
}

func readGeminiPackage(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var doc struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if json.Unmarshal(b, &doc) != nil || doc.Name != "@google/gemini-cli" {
		return ""
	}
	v, ok := normalizeSemver(doc.Version)
	if !ok {
		return ""
	}
	return v
}

func normalizeSemver(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || verRe.FindString(s) != s {
		return "", false
	}
	return strings.TrimPrefix(s, "v"), true
}

func commandStdout(bin string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	out, _ := cmd.Output()
	return string(out)
}

// findVersion returns the first semver token in stdout. Surrounding words on
// the same line are ignored. The caller passes stdout only, so stderr banners
// never reach this function.
func findVersion(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		m := verRe.FindString(line)
		if m == "" {
			continue
		}
		if v, ok := normalizeSemver(m); ok {
			return v
		}
	}
	return ""
}

// ClaudeCommandPath is the slash-command file Claude Code discovers.
func ClaudeCommandPath(home, project string, projectLevel bool) string {
	if projectLevel {
		return filepath.Join(project, ".claude", "commands", "oops.md")
	}
	return filepath.Join(home, ".claude", "commands", "oops.md")
}

// ClaudeCommandBody is the local /oops command description.
const ClaudeCommandBody = `---
description: Undo the last agent turn with oops
---

<!-- oops -->
Show recent oops turns, or undo one. The oops hook runs this locally; it is not sent to the model.

Usage: /oops [undo [N] | redo | diff [N]]
`

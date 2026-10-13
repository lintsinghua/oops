package snapshot

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// builtinDirExcludes are pruned even when they are not gitignored.
var builtinDirExcludes = map[string]struct{}{
	".git": {}, "node_modules": {}, ".venv": {}, "venv": {},
	"__pycache__": {}, ".mypy_cache": {}, ".pytest_cache": {}, ".ruff_cache": {},
	"target": {}, ".gradle": {}, ".next": {}, ".nuxt": {}, ".svelte-kit": {},
	".turbo": {}, ".parcel-cache": {}, ".cache": {}, "coverage": {}, ".terraform": {},
}

// gitignoreOnlyDirs are excluded only when a gitignore rule also matches.
var gitignoreOnlyDirs = map[string]struct{}{
	"dist": {}, "build": {},
}

// Rules decides which paths are captured.
type Rules struct {
	respectGit bool
	protect    []pattern
	oops       []scoped
	extraExcl  []pattern
	git        []scoped
}

type scoped struct {
	base string
	pats []pattern
}

type pattern struct {
	neg     bool
	dirOnly bool
	abs     bool
	text    string
}

// LoadRules reads .oopsignore, .gitignore files, and the built-in lists.
func LoadRules(root string, protect, extra []string, respectGit bool) (*Rules, error) {
	r := &Rules{respectGit: respectGit}
	for _, p := range append([]string{".env", ".env.*", ".envrc", "*.local", "*.local.*", ".dev.vars", "local.settings.json"}, protect...) {
		if pat, ok := parsePattern(p); ok {
			r.protect = append(r.protect, pat)
		}
	}
	for _, p := range extra {
		if pat, ok := parsePattern(p); ok {
			r.extraExcl = append(r.extraExcl, pat)
		}
	}
	if err := r.loadFile(root, "", ".oopsignore", true); err != nil {
		return nil, err
	}
	if respectGit {
		if err := r.loadGit(root); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Rules) loadGit(root string) error {
	candidates := []string{filepath.Join(root, ".git", "info", "exclude")}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".config", "git", "ignore"),
			filepath.Join(home, ".gitignore_global"),
		)
	}
	var global []pattern
	for _, c := range candidates {
		pats, err := readPatterns(c)
		if err != nil {
			return err
		}
		global = append(global, pats...)
	}
	if len(global) > 0 {
		r.git = append(r.git, scoped{base: "", pats: global})
	}
	return r.loadFile(root, "", ".gitignore", false)
}

func (r *Rules) loadFile(root, base, name string, oops bool) error {
	path := filepath.Join(root, filepath.FromSlash(base), name)
	pats, err := readPatterns(path)
	if err != nil || len(pats) == 0 {
		return err
	}
	s := scoped{base: base, pats: pats}
	if oops {
		r.oops = append(r.oops, s)
	} else {
		r.git = append(r.git, s)
	}
	return nil
}

// NoteDir records nested ignore files discovered while walking.
func (r *Rules) NoteDir(root, rel string) error {
	if err := r.loadFile(root, rel, ".oopsignore", true); err != nil {
		return err
	}
	if r.respectGit {
		if err := r.loadFile(root, rel, ".gitignore", false); err != nil {
			return err
		}
	}
	return nil
}

func readPatterns(path string) ([]pattern, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []pattern
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if pat, ok := parsePattern(line); ok {
			out = append(out, pat)
		}
	}
	return out, sc.Err()
}

func parsePattern(line string) (pattern, bool) {
	neg := false
	if strings.HasPrefix(line, `\!`) {
		line = line[1:]
	} else if strings.HasPrefix(line, "!") {
		neg = true
		line = line[1:]
	}
	if line == "" {
		return pattern{}, false
	}
	dirOnly := strings.HasSuffix(line, "/")
	line = strings.TrimSuffix(line, "/")
	abs := strings.HasPrefix(line, "/")
	line = strings.TrimPrefix(line, "/")
	return pattern{neg: neg, dirOnly: dirOnly, abs: abs || strings.Contains(line, "/"), text: line}, true
}

// Include reports whether a relative slash path is captured.
func (r *Rules) Include(rel string, isDir bool) bool {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	if rel == "" {
		return true
	}
	base := pathBase(rel)
	if strings.HasPrefix(base, ".oops-tmp-") {
		return false
	}
	if r.negated(rel, isDir) {
		return true
	}
	if r.oopsExcludes(rel, isDir) || matchAny(r.extraExcl, rel, isDir) {
		return false
	}
	if r.protected(rel, isDir) {
		return true
	}
	if builtinExclude(rel, isDir, r.gitIgnored(rel, isDir)) {
		return false
	}
	if r.respectGit && r.gitIgnored(rel, isDir) {
		return false
	}
	return true
}

func (r *Rules) negated(rel string, isDir bool) bool {
	return scopedNeg(r.oops, rel, isDir)
}

func (r *Rules) oopsExcludes(rel string, isDir bool) bool {
	for _, s := range r.oops {
		if matchScoped(s, rel, isDir, false) {
			return true
		}
	}
	return false
}

func (r *Rules) protected(rel string, isDir bool) bool {
	return matchAny(r.protect, rel, isDir)
}

func (r *Rules) gitIgnored(rel string, isDir bool) bool {
	ignored := false
	for _, s := range r.git {
		if ok, neg := matchScopedState(s, rel, isDir); ok {
			ignored = !neg
		}
	}
	return ignored
}

func scopedNeg(ss []scoped, rel string, isDir bool) bool {
	for _, s := range ss {
		if matchScoped(s, rel, isDir, true) {
			return true
		}
	}
	return false
}

func matchScoped(s scoped, rel string, isDir, wantNeg bool) bool {
	local := rel
	if s.base != "" {
		prefix := strings.TrimSuffix(s.base, "/") + "/"
		if rel != s.base && !strings.HasPrefix(rel, prefix) {
			return false
		}
		if strings.HasPrefix(rel, prefix) {
			local = strings.TrimPrefix(rel, prefix)
		}
	}
	var hit bool
	for _, p := range s.pats {
		if p.neg != wantNeg {
			continue
		}
		if matchPattern(p, local, isDir) {
			hit = !p.neg || wantNeg
			if !wantNeg && p.neg {
				hit = false
			}
			if wantNeg && p.neg && matchPattern(p, local, isDir) {
				return true
			}
			if !wantNeg && !p.neg && matchPattern(p, local, isDir) {
				hit = true
			}
		}
	}
	return hit && !wantNeg
}

func matchScopedState(s scoped, rel string, isDir bool) (bool, bool) {
	local := rel
	if s.base != "" {
		prefix := strings.TrimSuffix(s.base, "/") + "/"
		if rel != strings.TrimSuffix(s.base, "/") && !strings.HasPrefix(rel, prefix) {
			return false, false
		}
		if strings.HasPrefix(rel, prefix) {
			local = strings.TrimPrefix(rel, prefix)
		}
	}
	matched := false
	neg := false
	for _, p := range s.pats {
		if matchPattern(p, local, isDir) {
			matched = true
			neg = p.neg
		}
	}
	return matched, neg
}

func matchAny(ps []pattern, rel string, isDir bool) bool {
	for _, p := range ps {
		if p.neg {
			continue
		}
		if matchPattern(p, rel, isDir) || matchPattern(p, pathBase(rel), isDir) {
			return true
		}
	}
	return false
}

func matchPattern(p pattern, path string, isDir bool) bool {
	if p.dirOnly && !isDir {
		return false
	}
	path = strings.TrimPrefix(path, "/")
	pat := p.text
	if strings.Contains(pat, "/") || p.abs {
		return globMatch(pat, path)
	}
	if globMatch(pat, path) || globMatch(pat, pathBase(path)) {
		return true
	}
	for _, part := range strings.Split(path, "/") {
		if globMatch(pat, part) {
			return true
		}
	}
	return false
}

func globMatch(pattern, name string) bool {
	ok, err := pathMatch(pattern, name)
	return err == nil && ok
}

func pathMatch(pattern, name string) (bool, error) {
	// Translate ** into a form filepath-style matching cannot express, via a small recursive matcher.
	return wildMatch(pattern, name), nil
}

func wildMatch(pattern, name string) bool {
	for {
		if pattern == "" {
			return name == ""
		}
		if strings.HasPrefix(pattern, "**") {
			rest := strings.TrimPrefix(pattern, "**")
			rest = strings.TrimPrefix(rest, "/")
			if rest == "" {
				return true
			}
			if wildMatch(rest, name) {
				return true
			}
			for i := 0; i < len(name); i++ {
				if name[i] == '/' && wildMatch(rest, name[i+1:]) {
					return true
				}
			}
			return false
		}
		if name == "" {
			return false
		}
		switch pattern[0] {
		case '*':
			pattern = pattern[1:]
			for {
				if wildMatch(pattern, name) {
					return true
				}
				if name == "" || name[0] == '/' {
					return false
				}
				name = name[1:]
			}
		case '?':
			if name[0] == '/' {
				return false
			}
			pattern = pattern[1:]
			name = name[1:]
		default:
			if pattern[0] != name[0] {
				return false
			}
			pattern = pattern[1:]
			name = name[1:]
		}
	}
}

func pathBase(rel string) string {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[i+1:]
	}
	return rel
}

func builtinExclude(rel string, isDir, gitIgnored bool) bool {
	base := pathBase(rel)
	if base == ".DS_Store" {
		return true
	}
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		if _, ok := builtinDirExcludes[part]; ok {
			return true
		}
		if _, ok := gitignoreOnlyDirs[part]; ok && gitIgnored {
			return true
		}
	}
	return false
}

// PruneDir reports whether a directory can be skipped entirely.
func (r *Rules) PruneDir(rel string) bool {
	if !r.Include(rel, true) {
		// Still descend when a force-include could live underneath an oops negation,
		// but builtin excludes are always pruned.
		base := pathBase(rel)
		if _, ok := builtinDirExcludes[base]; ok {
			return true
		}
		for _, part := range strings.Split(rel, "/") {
			if _, ok := builtinDirExcludes[part]; ok {
				return true
			}
		}
		if r.oopsExcludes(rel, true) && !r.negated(rel, true) {
			return true
		}
		return false
	}
	return false
}

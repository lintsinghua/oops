package engine

import "strings"

func filterPaths(paths, patterns []string) (matched, missing []string) {
	seen := map[string]struct{}{}
	for _, pat := range patterns {
		hit := false
		for _, p := range paths {
			if matchPath(pat, p) {
				if _, ok := seen[p]; !ok {
					matched = append(matched, p)
					seen[p] = struct{}{}
				}
				hit = true
			}
		}
		if !hit {
			missing = append(missing, pat)
		}
	}
	return matched, missing
}

func matchPath(pattern, path string) bool {
	pattern = strings.TrimPrefix(strings.ReplaceAll(pattern, "\\", "/"), "./")
	path = strings.ReplaceAll(path, "\\", "/")
	if pattern == path {
		return true
	}
	if strings.HasSuffix(pattern, "/") && strings.HasPrefix(path, pattern) {
		return true
	}
	return wild(pattern, path) || wild(pattern, base(path))
}

func base(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func wild(pattern, name string) bool {
	for {
		if pattern == "" {
			return name == ""
		}
		if strings.HasPrefix(pattern, "**") {
			rest := strings.TrimPrefix(strings.TrimPrefix(pattern, "**"), "/")
			if rest == "" {
				return true
			}
			if wild(rest, name) {
				return true
			}
			for i := 0; i < len(name); i++ {
				if name[i] == '/' && wild(rest, name[i+1:]) {
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
				if wild(pattern, name) {
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
			pattern, name = pattern[1:], name[1:]
		default:
			if pattern[0] != name[0] {
				return false
			}
			pattern, name = pattern[1:], name[1:]
		}
	}
}

// Package platform holds OS-specific path and file behavior.
package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// CaseSensitive reports whether workspace paths are compared case-sensitively.
// Darwin and Windows are treated as insensitive, matching typical APFS and NTFS volumes.
func CaseSensitive() bool {
	return runtime.GOOS == "linux" || runtime.GOOS == "android" || runtime.GOOS == "freebsd"
}

// Key normalizes a relative path for comparison.
func Key(rel string) string {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." {
		return ""
	}
	rel = norm.NFC.String(rel)
	if !CaseSensitive() {
		rel = strings.ToLower(rel)
	}
	return rel
}

// Same reports whether two relative paths refer to the same file on this OS.
func Same(a, b string) bool {
	return Key(a) == Key(b)
}

// WorkspaceID is the first 16 hex chars of sha256(canonical real path).
func WorkspaceID(path string) (string, string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	real := abs
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		real = resolved
	}
	canon := filepath.ToSlash(filepath.Clean(real))
	if !CaseSensitive() {
		canon = strings.ToLower(canon)
	}
	sum := sha256.Sum256([]byte(canon))
	return hex.EncodeToString(sum[:])[:16], abs, nil
}

// FindRoot walks up from start to a directory containing .git.
// Without .git, start itself is the root.
func FindRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	cur := abs
	for {
		if st, err := os.Lstat(filepath.Join(cur, ".git")); err == nil && (st.IsDir() || st.Mode().IsRegular()) {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs, nil
		}
		cur = parent
	}
}

// ForbiddenRoot reports whether path must not be used as a workspace root.
func ForbiddenRoot(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return true
	}
	clean := filepath.Clean(abs)
	if clean == string(filepath.Separator) || clean == "/" {
		return true
	}
	home, err := os.UserHomeDir()
	if err == nil && sameDir(clean, home) {
		return true
	}
	switch runtime.GOOS {
	case "windows":
		if isDriveRoot(clean) {
			return true
		}
		win := strings.ToLower(filepath.Clean(os.Getenv("SystemRoot")))
		if win != "" && sameDir(clean, win) {
			return true
		}
	default:
		switch clean {
		case "/usr", "/bin", "/sbin", "/etc", "/var", "/lib", "/lib64", "/opt", "/boot", "/proc", "/sys", "/dev":
			return true
		}
	}
	return false
}

func isDriveRoot(path string) bool {
	vol := filepath.VolumeName(path)
	rest := strings.TrimPrefix(path, vol)
	rest = strings.Trim(rest, `\/`)
	return vol != "" && rest == ""
}

func sameDir(a, b string) bool {
	if !CaseSensitive() {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// LongPath applies the Windows extended-length prefix when needed.
func LongPath(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	if strings.HasPrefix(path, `\\?\`) || strings.HasPrefix(path, `\\.\`) {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return `\\?\` + abs
}

// SafeRel joins root and a slash-separated relative path after rejecting escapes.
func SafeRel(root, rel string) (string, error) {
	if rel == "" || strings.ContainsRune(rel, '\x00') {
		return "", errors.New("empty path")
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) {
		return "", fmt.Errorf("absolute path %q", rel)
	}
	slash := filepath.ToSlash(rel)
	if slash == ".." || strings.HasPrefix(slash, "../") || strings.Contains(slash, "/../") {
		return "", fmt.Errorf("path %q escapes the workspace", rel)
	}
	for _, part := range strings.Split(slash, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("path %q escapes the workspace", rel)
		}
		if isDeviceName(part) {
			return "", fmt.Errorf("path %q uses a reserved device name", rel)
		}
	}
	full := filepath.Join(root, filepath.FromSlash(slash))
	relCheck, err := filepath.Rel(root, full)
	if err != nil || strings.HasPrefix(relCheck, "..") {
		return "", fmt.Errorf("path %q escapes the workspace", rel)
	}
	if err := rejectEscapingSymlink(root, full); err != nil {
		return "", err
	}
	return full, nil
}

func isDeviceName(part string) bool {
	name := part
	if i := strings.IndexByte(name, '.'); i >= 0 {
		name = name[:i]
	}
	switch strings.ToUpper(name) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
}

func rejectEscapingSymlink(root, full string) error {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return err
	}
	cur := root
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		cur = filepath.Join(cur, part)
		st, err := os.Lstat(cur)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if st.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(cur)
		if err != nil {
			return err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(cur), target)
		}
		resolved, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		inside, err := filepath.Rel(root, resolved)
		if err != nil || strings.HasPrefix(inside, "..") {
			return fmt.Errorf("symlink %s points outside the workspace", cur)
		}
	}
	return nil
}

// securedDirs remembers directories this process has already made private.
// Resetting the Windows ACL on every file in a batch dominated undo time.
var securedDirs sync.Map

// SecureMkdir creates a directory private to the current user.
func SecureMkdir(path string) error {
	path = filepath.Clean(path)
	if _, ok := securedDirs.Load(path); ok {
		return nil
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o700); err != nil && runtime.GOOS != "windows" {
		return err
	}
	if err := secureDir(path); err != nil {
		return err
	}
	securedDirs.Store(path, struct{}{})
	return nil
}

// WriteFileAtomic writes data to path via a temp file, fsync, and rename.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	return writeFileAtomic(path, data, mode, true)
}

// WriteFileAtomicUnsynced writes via a temp file and rename. The caller syncs
// the directory once after a batch. Per-file fsync makes a 1,000-file undo
// miss the PRD limit on Windows.
func WriteFileAtomicUnsynced(path string, data []byte, mode os.FileMode) error {
	return writeFileAtomic(path, data, mode, false)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode, sync bool) error {
	if err := SecureMkdir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".oops-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if sync {
		if err := tmp.Sync(); err != nil {
			_ = tmp.Close()
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if mode == 0 {
		mode = 0o600
	}
	if err := os.Chmod(tmpName, mode); err != nil && runtime.GOOS != "windows" {
		return err
	}
	if err := renameReplace(tmpName, path); err != nil {
		return err
	}
	ok = true
	return nil
}

// SyncDir flushes directory entries. Windows directory handles often reject
// this; a failure there is ignored because the bytes are already renamed.
func SyncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Sync(); err != nil && runtime.GOOS != "windows" {
		return err
	}
	return nil
}

// CopyFileAtomic copies src bytes into dst inside the same directory when possible.
func CopyReaderAtomic(path string, r io.Reader, mode os.FileMode) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, data, mode)
}

// IsNotExist reports a missing file.
func IsNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

// ExecBit keeps the executable bits that oops restores. Windows ignores them.
func ExecBit(mode uint32) uint32 {
	if runtime.GOOS == "windows" {
		return 0
	}
	return mode & 0o111
}

// DisplayPath shortens a path with ~ when it is under home.
func DisplayPath(path, home string) string {
	if home == "" {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	if rel == "." {
		return "~"
	}
	return "~/" + filepath.ToSlash(rel)
}

// HasUnicodeSpace is a tiny helper used by tests to keep the unicode import live
// when a caller only needs Key.
func HasUnicodeSpace(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

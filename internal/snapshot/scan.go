package snapshot

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/lintsinghua/oops/internal/platform"
	"github.com/lintsinghua/oops/internal/store"
)

// Options controls one scan.
type Options struct {
	MaxFileSize int64
	Timeout     time.Duration
	Protect     []string
	Exclude     []string
	RespectGit  bool
	Prev        *Cache
	Now         time.Time
}

// Result is a completed or partial snapshot.
type Result struct {
	Root     store.ID
	Files    map[string]Meta
	Skipped  []Meta
	Partial  bool
	Unstable []string
	Cache    *Cache
	// Changed is true when any file was not a stat-cache hit.
	Changed bool
}

type memNode struct {
	entry Entry
	kids  map[string]*memNode
}

// Scan walks root and writes changed objects into st.
func Scan(ctx context.Context, st *store.Store, root string, opt Options) (*Result, error) {
	if opt.MaxFileSize <= 0 {
		opt.MaxFileSize = 50 << 20
	}
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}
	rules, err := LoadRules(root, opt.Protect, opt.Exclude, opt.RespectGit)
	if err != nil {
		return nil, err
	}
	if opt.Prev == nil {
		opt.Prev = &Cache{Entries: map[string]CacheEntry{}}
	}
	// The deadline is wall-clock time from the start of this scan. opt.Now is the
	// caller's logical clock and can be far behind the filesystem.
	deadline := time.Time{}
	if opt.Timeout > 0 {
		deadline = time.Now().Add(opt.Timeout)
	}
	rootNode := &memNode{kids: map[string]*memNode{}, entry: Entry{Kind: KindDir, Name: ""}}
	// ScanTime is wall-clock so it can be compared with filesystem mtimes.
	// opt.Now is the logical clock used by callers and may not match mtime.
	res := &Result{Files: map[string]Meta{}, Cache: &Cache{Entries: map[string]CacheEntry{}, ScanTime: time.Now().UnixNano()}}
	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if workers < 1 {
		workers = 1
	}
	partial, err := walk(ctx, st, root, "", rootNode, rules, opt, res, deadline, workers)
	if err != nil {
		return nil, err
	}
	res.Partial = partial
	if partial {
		fillUnvisited(opt.Prev, res)
	}
	if partial || res.Changed || !cacheComplete(opt.Prev, res.Cache) || opt.Prev.LastRoot.IsZero() {
		id, err := writeTree(st, rootNode)
		if err != nil {
			return nil, err
		}
		res.Root = id
		res.Changed = true
	} else {
		res.Root = opt.Prev.LastRoot
	}
	res.Files = map[string]Meta{}
	flattenNode(rootNode, "", res.Files)
	res.Cache.LastRoot = res.Root
	return res, nil
}

func walk(ctx context.Context, st *store.Store, root, rel string, node *memNode, rules *Rules, opt Options, res *Result, deadline time.Time, workers int) (bool, error) {
	if err := ctx.Err(); err != nil {
		return true, nil
	}
	if !deadline.IsZero() && time.Now().After(deadline) {
		return true, nil
	}
	if rel != "" {
		if err := rules.NoteDir(root, rel); err != nil {
			return false, err
		}
	}
	dir := root
	if rel != "" {
		var err error
		dir, err = platform.SafeRel(root, rel)
		if err != nil {
			return false, err
		}
	}
	entries, err := os.ReadDir(platform.LongPath(dir))
	if err != nil {
		return false, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	type job struct {
		name string
		info os.FileInfo
		path string
		key  string
	}
	var jobs []job
	var dirs []os.DirEntry
	included := 0
	for _, de := range entries {
		name := de.Name()
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}
		mode := de.Type()
		if mode == 0 {
			info, err := os.Lstat(platform.LongPath(filepath.Join(dir, name)))
			if err != nil {
				continue
			}
			mode = info.Mode()
		}
		isDir := mode.IsDir()
		if mode&os.ModeSocket != 0 || mode&os.ModeDevice != 0 || mode&os.ModeNamedPipe != 0 {
			continue
		}
		if isDir && rules.PruneDir(childRel) {
			continue
		}
		if !rules.Include(childRel, isDir || mode&os.ModeSymlink != 0 && false) && !isDir {
			if !rules.Include(childRel, false) {
				continue
			}
		}
		if isDir && !rules.Include(childRel, true) {
			// Directory itself is excluded and not pruned only when we still
			// need to look for protected files. Descend in that case.
			if rules.PruneDir(childRel) {
				continue
			}
		}
		if !isDir && !rules.Include(childRel, false) && mode&os.ModeSymlink == 0 {
			continue
		}
		if mode&os.ModeSymlink != 0 {
			if !rules.Include(childRel, false) {
				continue
			}
		}
		included++
		if included > 200_000 {
			return true, nil
		}
		if isDir && mode&os.ModeSymlink == 0 {
			dirs = append(dirs, de)
			continue
		}
		jobs = append(jobs, job{name: name, path: childRel, key: platform.Key(childRel)})
	}
	if len(jobs) > 0 {
		nworkers := workers
		if nworkers > len(jobs) {
			nworkers = len(jobs)
		}
		if nworkers < 1 {
			nworkers = 1
		}
		ch := make(chan int)
		var swg sync.WaitGroup
		for w := 0; w < nworkers; w++ {
			swg.Add(1)
			go func() {
				defer swg.Done()
				for i := range ch {
					info, err := os.Lstat(platform.LongPath(filepath.Join(dir, jobs[i].name)))
					if err == nil {
						jobs[i].info = info
					}
				}
			}()
		}
		for i := range jobs {
			ch <- i
		}
		close(ch)
		swg.Wait()
		kept := jobs[:0]
		for _, j := range jobs {
			if j.info != nil {
				kept = append(kept, j)
			}
		}
		jobs = kept
	}

	var misses []job
	for _, j := range jobs {
		if entry, cached, ok := reuseCached(j.info, opt, j.key); ok {
			entry.Name = j.name
			node.kids[j.name] = &memNode{entry: entry}
			res.Cache.Entries[j.key] = cached
			continue
		}
		res.Changed = true
		misses = append(misses, j)
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	var first error
	partial := false
	for _, j := range misses {
		if !deadline.IsZero() && time.Now().After(deadline) {
			partial = true
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(j job) {
			defer wg.Done()
			defer func() { <-sem }()
			entry, cached, skip, unstable, err := hashOne(st, root, j.path, j.info, opt)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && first == nil {
				first = err
				return
			}
			if unstable {
				res.Unstable = append(res.Unstable, j.path)
			}
			entry.Name = j.name
			node.kids[j.name] = &memNode{entry: entry}
			if skip {
				res.Skipped = append(res.Skipped, metaFrom(j.path, entry))
			}
			res.Cache.Entries[j.key] = cached
		}(j)
	}
	wg.Wait()
	if first != nil {
		return false, first
	}
	for _, de := range dirs {
		if !deadline.IsZero() && time.Now().After(deadline) {
			return true, nil
		}
		name := de.Name()
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}
		child := &memNode{kids: map[string]*memNode{}, entry: Entry{Kind: KindDir, Name: name, Mode: 0o755}}
		p, err := walk(ctx, st, root, childRel, child, rules, opt, res, deadline, workers)
		if err != nil {
			return false, err
		}
		if p {
			partial = true
		}
		info, err := os.Lstat(filepath.Join(dir, name))
		if err == nil {
			child.entry.Mode = uint32(info.Mode().Perm())
			child.entry.Mtime = info.ModTime().UnixNano()
		}
		node.kids[name] = child
		res.Cache.Entries[platform.Key(childRel)] = CacheEntry{Mtime: child.entry.Mtime, Mode: child.entry.Mode, Kind: KindDir}
	}
	return partial, nil
}

// reuseCached reports a stat-cache hit. The conditions match hashOne.
func reuseCached(info os.FileInfo, opt Options, key string) (Entry, CacheEntry, bool) {
	if opt.Prev == nil || info.Mode()&os.ModeSymlink != 0 {
		return Entry{}, CacheEntry{}, false
	}
	prev, ok := opt.Prev.Entries[key]
	if !ok || prev.ID.IsZero() {
		return Entry{}, CacheEntry{}, false
	}
	mode := uint32(info.Mode().Perm())
	mtime := info.ModTime().UnixNano()
	racy := opt.Prev.ScanTime > 0 && mtime >= opt.Prev.ScanTime-int64(time.Second)
	if racy || prev.Size != info.Size() || prev.Mtime != mtime || prev.Mode != mode {
		return Entry{}, CacheEntry{}, false
	}
	entry := Entry{Kind: KindFile, Mode: mode, Size: info.Size(), Mtime: mtime, ID: prev.ID}
	return entry, prev, true
}

func cacheComplete(prev, cur *Cache) bool {
	if prev == nil || cur == nil || len(prev.Entries) != len(cur.Entries) {
		return false
	}
	for k := range prev.Entries {
		if _, ok := cur.Entries[k]; !ok {
			return false
		}
	}
	return true
}

func flattenNode(n *memNode, prefix string, out map[string]Meta) {
	names := make([]string, 0, len(n.kids))
	for name := range n.kids {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ch := n.kids[name]
		p := name
		if prefix != "" {
			p = prefix + "/" + name
		}
		out[p] = metaFrom(p, ch.entry)
		if ch.entry.Kind == KindDir {
			flattenNode(ch, p, out)
		}
	}
}

func hashOne(st *store.Store, root, rel string, info os.FileInfo, opt Options) (Entry, CacheEntry, bool, bool, error) {
	mode := uint32(info.Mode().Perm())
	entry := Entry{Kind: KindFile, Mode: mode, Size: info.Size(), Mtime: info.ModTime().UnixNano()}
	key := platform.Key(rel)
	prev, hasPrev := opt.Prev.Entries[key]
	if info.Mode()&os.ModeSymlink != 0 {
		full, err := platform.SafeRel(root, rel)
		if err != nil {
			return entry, CacheEntry{}, false, false, err
		}
		target, err := os.Readlink(platform.LongPath(full))
		if err != nil {
			return entry, CacheEntry{}, false, false, err
		}
		entry.Kind = KindSymlink
		entry.Target = target
		entry.Size = int64(len(target))
		id, err := st.Put([]byte(target))
		if err != nil {
			return entry, CacheEntry{}, false, false, err
		}
		entry.ID = id
		return entry, CacheEntry{Size: entry.Size, Mtime: entry.Mtime, Mode: mode, ID: id, Kind: KindSymlink}, false, false, nil
	}
	racy := hasPrev && opt.Prev.ScanTime > 0 && entry.Mtime >= opt.Prev.ScanTime-int64(time.Second)
	if hasPrev && prev.Size == entry.Size && prev.Mtime == entry.Mtime && prev.Mode == mode && !racy && !prev.ID.IsZero() {
		entry.ID = prev.ID
		return entry, prev, false, false, nil
	}
	if entry.Size > opt.MaxFileSize {
		entry.Skipped = "too_large"
		ce := CacheEntry{Size: entry.Size, Mtime: entry.Mtime, Mode: mode, Kind: KindFile}
		return entry, ce, true, false, nil
	}
	full, err := platform.SafeRel(root, rel)
	if err != nil {
		return entry, CacheEntry{}, false, false, err
	}
	var raw []byte
	for attempt := 0; attempt < 3; attempt++ {
		before, err := os.Lstat(platform.LongPath(full))
		if err != nil {
			return entry, CacheEntry{}, false, false, err
		}
		f, err := os.Open(platform.LongPath(full))
		if err != nil {
			return entry, CacheEntry{}, false, false, err
		}
		raw, err = io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			return entry, CacheEntry{}, false, false, err
		}
		after, err := os.Lstat(platform.LongPath(full))
		if err != nil {
			return entry, CacheEntry{}, false, false, err
		}
		if before.Size() == after.Size() && before.ModTime().Equal(after.ModTime()) {
			entry.Size = after.Size()
			entry.Mtime = after.ModTime().UnixNano()
			entry.Mode = uint32(after.Mode().Perm())
			break
		}
		if attempt == 2 {
			entry.Skipped = "unstable"
			return entry, CacheEntry{Size: entry.Size, Mtime: entry.Mtime, Mode: entry.Mode, Kind: KindFile}, true, true, nil
		}
	}
	id, err := st.Put(raw)
	if err != nil {
		return entry, CacheEntry{}, false, false, err
	}
	entry.ID = id
	return entry, CacheEntry{Size: entry.Size, Mtime: entry.Mtime, Mode: entry.Mode, ID: id, Kind: KindFile}, false, false, nil
}

func writeTree(st *store.Store, n *memNode) (store.ID, error) {
	entries := make([]Entry, 0, len(n.kids))
	names := make([]string, 0, len(n.kids))
	for name := range n.kids {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ch := n.kids[name]
		if ch.entry.Kind == KindDir {
			id, err := writeTree(st, ch)
			if err != nil {
				return store.ID{}, err
			}
			ch.entry.ID = id
			ch.entry.Kind = KindDir
		}
		entries = append(entries, ch.entry)
	}
	raw, err := EncodeTree(entries)
	if err != nil {
		return store.ID{}, err
	}
	return st.Put(raw)
}

func fillUnvisited(prev *Cache, res *Result) {
	// Partial scans keep the cache entries gathered so far.
	// Paths not visited remain available via the previous snapshot at a higher layer.
	_ = prev
}

func metaFrom(path string, e Entry) Meta {
	return Meta{Path: path, Kind: e.Kind, Mode: e.Mode, Size: e.Size, ID: e.ID, Target: e.Target, Skipped: e.Skipped, Mtime: e.Mtime, Ino: e.Ino}
}

// Flatten expands a tree id into slash-separated paths.
func Flatten(st *store.Store, id store.ID) (map[string]Meta, error) {
	out := map[string]Meta{}
	if id.IsZero() {
		return out, nil
	}
	var walk func(id store.ID, prefix string) error
	walk = func(id store.ID, prefix string) error {
		raw, err := st.Get(id)
		if err != nil {
			return err
		}
		entries, err := DecodeTree(raw)
		if err != nil {
			return err
		}
		for _, e := range entries {
			p := e.Name
			if prefix != "" {
				p = prefix + "/" + e.Name
			}
			out[p] = metaFrom(p, e)
			if e.Kind == KindDir && !e.ID.IsZero() {
				if err := walk(e.ID, p); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(id, ""); err != nil {
		return nil, err
	}
	return out, nil
}

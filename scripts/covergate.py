#!/usr/bin/env python3
"""Fail unless overall coverage is at least 70% and store, snapshot, and restore are each at least 85%.

go test -coverpkg=./... writes one block per package test, so counts for the same block are summed.
"""

import sys

CORE = {
    "github.com/lintsinghua/oops/internal/store",
    "github.com/lintsinghua/oops/internal/snapshot",
    "github.com/lintsinghua/oops/internal/restore",
}


def main() -> int:
    path = sys.argv[1] if len(sys.argv) > 1 else "coverage.out"
    blocks = {}
    with open(path, encoding="utf-8") as f:
        for line in f:
            if line.startswith("mode:"):
                continue
            left, count_s = line.rsplit(" ", 1)
            pathspan, nstmt_s = left.rsplit(" ", 1)
            nstmt, count = int(nstmt_s), int(count_s)
            prev = blocks.get(pathspan)
            if prev is None:
                blocks[pathspan] = (nstmt, count)
            else:
                blocks[pathspan] = (nstmt, prev[1] + count)
    by = {}
    total_s = total_c = 0
    for pathspan, (nstmt, count) in blocks.items():
        file = pathspan.split(":", 1)[0]
        pkg = file.rsplit("/", 1)[0]
        covered = nstmt if count > 0 else 0
        s, c = by.get(pkg, (0, 0))
        by[pkg] = (s + nstmt, c + covered)
        total_s += nstmt
        total_c += covered
    ok = True
    overall = 100 * total_c / total_s if total_s else 0
    print(f"overall {overall:.2f}% ({total_c}/{total_s}) threshold 70%")
    if total_c * 100 < total_s * 70:
        ok = False
    for pkg in sorted(CORE):
        s, c = by.get(pkg, (0, 0))
        pct = 100 * c / s if s else 0
        print(f"{pkg} {pct:.2f}% ({c}/{s}) threshold 85%")
        if c * 100 < s * 85:
            ok = False
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())

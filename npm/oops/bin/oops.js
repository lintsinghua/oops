#!/usr/bin/env node
"use strict";

var spawnSync = require("child_process").spawnSync;
var existsSync = require("fs").existsSync;
var path = require("path");

var key = process.platform + "-" + process.arch;
var pkg = "@lintsinghua/oops-" + key;
var binName = process.platform === "win32" ? "oops.exe" : "oops";

function resolveBin() {
	var pkgJson;
	try {
		pkgJson = require.resolve(pkg + "/package.json");
	} catch (err) {
		return null;
	}
	return path.join(path.dirname(pkgJson), "bin", binName);
}

var bin = resolveBin();
if (!bin || !existsSync(bin)) {
	process.stderr.write(
		"oops: no binary for " + key + ".\n" +
			"Expected optional dependency " + pkg + ".\n"
	);
	process.exit(1);
}

var result = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
	process.stderr.write(result.error.message + "\n");
	process.exit(1);
}
if (result.signal) {
	process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);

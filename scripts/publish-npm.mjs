#!/usr/bin/env node
// Publish @lintsinghua/oops and its platform packages from goreleaser dist/.
// Usage: NPM_TOKEN=... node scripts/publish-npm.mjs [version]
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

const platforms = [
	["linux", "amd64", "linux", "x64", "oops", "oops-linux-x64"],
	["linux", "arm64", "linux", "arm64", "oops", "oops-linux-arm64"],
	["darwin", "amd64", "darwin", "x64", "oops", "oops-darwin-x64"],
	["darwin", "arm64", "darwin", "arm64", "oops", "oops-darwin-arm64"],
	["windows", "amd64", "win32", "x64", "oops.exe", "oops-win32-x64"],
	["windows", "arm64", "win32", "arm64", "oops.exe", "oops-win32-arm64"],
];

function fail(message) {
	process.stderr.write(message + "\n");
	process.exit(1);
}

function readJSON(file) {
	return JSON.parse(fs.readFileSync(file, "utf8"));
}

function writeJSON(file, value) {
	fs.writeFileSync(file, JSON.stringify(value, null, 2) + "\n");
}

function version() {
	const arg = process.argv[2];
	const raw = arg || process.env.GITHUB_REF_NAME || "";
	const name = raw.replace(/^refs\/tags\//, "").replace(/^v/, "");
	if (!name || /\s/.test(name)) {
		fail("usage: node scripts/publish-npm.mjs <version>  (or set GITHUB_REF_NAME)");
	}
	return name;
}

function findBinary(goos, goarch, binName) {
	const dist = path.join(root, "dist");
	if (!fs.existsSync(dist)) fail("dist/ not found; run goreleaser first");
	const hits = [];
	const walk = (dir) => {
		for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
			const full = path.join(dir, ent.name);
			if (ent.isDirectory()) walk(full);
			else if (ent.name === binName && full.includes(`${goos}_${goarch}`)) hits.push(full);
		}
	};
	walk(dist);
	if (hits.length === 0) fail(`no ${binName} for ${goos}/${goarch} under dist/`);
	hits.sort((a, b) => a.length - b.length);
	return hits[0];
}

function copyTree(src, dest) {
	fs.mkdirSync(dest, { recursive: true });
	for (const ent of fs.readdirSync(src, { withFileTypes: true })) {
		const from = path.join(src, ent.name);
		const to = path.join(dest, ent.name);
		if (ent.isDirectory()) copyTree(from, to);
		else if (ent.isFile()) fs.copyFileSync(from, to);
	}
}

const releaseVersion = version();
const token = process.env.NPM_TOKEN || "";
if (!token) fail("NPM_TOKEN is empty");

const stage = fs.mkdtempSync(path.join(os.tmpdir(), "oops-npm-"));
const npmrc = path.join(stage, ".npmrc");
fs.writeFileSync(npmrc, `//registry.npmjs.org/:_authToken=${token}\n`, { mode: 0o600 });

const staged = [];
for (const [goos, goarch, , , binName, dirName] of platforms) {
	const dest = path.join(stage, dirName);
	copyTree(path.join(root, "npm", dirName), dest);
	const pkg = readJSON(path.join(dest, "package.json"));
	pkg.version = releaseVersion;
	writeJSON(path.join(dest, "package.json"), pkg);
	const binDir = path.join(dest, "bin");
	fs.mkdirSync(binDir, { recursive: true });
	const binPath = path.join(binDir, binName);
	fs.copyFileSync(findBinary(goos, goarch, binName), binPath);
	fs.chmodSync(binPath, 0o755);
	staged.push(dest);
}

const mainDest = path.join(stage, "oops");
copyTree(path.join(root, "npm", "oops"), mainDest);
const mainPkg = readJSON(path.join(mainDest, "package.json"));
mainPkg.version = releaseVersion;
for (const name of Object.keys(mainPkg.optionalDependencies || {})) {
	mainPkg.optionalDependencies[name] = releaseVersion;
}
writeJSON(path.join(mainDest, "package.json"), mainPkg);
fs.chmodSync(path.join(mainDest, "bin", "oops.js"), 0o755);
staged.push(mainDest);

for (const dir of staged) {
	const name = readJSON(path.join(dir, "package.json")).name;
	process.stdout.write(`publishing ${name}@${releaseVersion}\n`);
	execFileSync("npm", ["publish", "--access", "public", "--userconfig", npmrc], {
		cwd: dir,
		stdio: "inherit",
	});
}

# Security Policy

## Supported versions

| Version | Supported |
| --- | --- |
| Latest tagged release | Yes |
| Older releases | No |

Before the first release, fixes land on the default branch.

## Threat model

oops is a local tool. The commands do not open network connections, send telemetry, or check for updates. `oops version` prints the version, commit, and date that were compiled in. `oops version --check` is not implemented.

The sensitive part is what snapshots keep on disk:

- Protected files are copied even when gitignore hides them. The default list includes `.env`, `.env.*`, `.envrc`, `*.local`, `*.local.*`, `.dev.vars`, and `local.settings.json`. A deleted `.env` can be restored, which means the secret now exists both in the workspace and in the data directory.
- File bytes are SHA-256 content-addressed and zstd-compressed under the data directory (`~/.local/share/oops`, `%LOCALAPPDATA%\oops`, or `OOPS_HOME`). They are not encrypted. Anyone who can read that directory can read the secrets in it, including root and any backup that copies your home directory.
- On Unix, new data directories are mode `0700` and object files are mode `0600`. On Windows, the data directory ACL grants the current user only.
- Turn metadata in `meta.db` includes the agent name and the first line of the prompt, up to 200 characters, with a few common secret shapes redacted. Set `privacy.record_prompts = false` in the config file to store no prompt text. Redaction is not a guarantee; do not put secrets in the prompt if you need them absent from `meta.db`.
- Hook processes run the `oops` binary with the permissions of the agent. They exit 0 even when a snapshot fails. They do not send the workspace contents anywhere.
- Undo writes file bytes back into the workspace. `--force` overwrites edits made after the turn. The previous bytes are kept in a safety snapshot first, still inside the data directory.

Exclude a file in `.oopsignore` if it should not be snapshotted. `oops gc --all --global` deletes snapshot data for every workspace oops knows about. Removing the data directory deletes the rest.

This build does not sign release artifacts with cosign and does not publish an SBOM or SLSA provenance. Check `checksums.txt` (sha256) against the archive you downloaded.

## Reporting a vulnerability

Report vulnerabilities privately through GitHub Security Advisories:

<https://github.com/lintsinghua/oops/security/advisories/new>

Please include a description, the version or commit you tested, and a way to reproduce the issue if you have one. Do not open a public issue for a security report.

We will acknowledge reports that we can reproduce and use the advisory to coordinate a fix.

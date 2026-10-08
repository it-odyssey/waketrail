# WakeTrail v1 Hardening

This document tracks correctness, trust, reliability, compatibility,
and release-readiness work required before the WakeTrail v1 release.

This is not the general feature roadmap.

WakeTrail v1 feature work may continue in parallel, but items marked
**Release Blocker** must be resolved before v1 is tagged.

---

## Status

- [ ] Not started
- [~] In progress
- [x] Completed
- [?] Needs verification

Priority:

- **Release Blocker** — must be resolved before v1
- **Important** — strongly preferred for v1
- **Review** — investigate and decide whether action is required
- **Post-v1** — explicitly deferred

---

# 1. Security and Privacy

## Release Blockers

### [x] Expand secret redaction coverage

Implemented best-effort new-write redaction for the common formats below.
The SQLite write boundary also sanitizes command, output, and timeline text;
manual note confirmations use redacted text. Synthetic persistence tests verify
that denied output is not read and common secrets do not reach new rows.
Existing recordings are not rewritten. See `docs/SECURITY-PRIVACY.md`.

Test and support at minimum:

- Environment variables containing sensitive names
  - `GITHUB_TOKEN`
  - `AWS_SESSION_TOKEN`
  - `POSTGRES_PASSWORD`
  - `DB_PASSWORD`
- JSON/YAML sensitive keys
- URL credentials
- Common CLI password/token flags
- Known token prefixes
- Authorization headers
- JWT-like tokens
- PEM/private-key blocks

Requirements:

- Add table-driven positive and negative tests.
- Avoid obvious false positives.
- Redaction must occur before persistence.
- Document that redaction is best-effort, not a security guarantee.

---

### [x] Add capture deny rules for secret-prone commands

Implemented metadata-only capture for secret-prone inspection commands and
arbitrary web response bodies. Classification is rechecked by the recorder,
so stale hooks and direct `record --capture-mode output` calls cannot bypass it.
Complex shell syntax also defaults to metadata-only. See the documented policy
and regression matrix in `internal/capture/privacy_test.go`.

Review and restrict output capture for:

- `kubectl get secret`
- Secret resources emitted as YAML/JSON
- `docker inspect` and `docker compose config`
- `terraform output`
- `terraform show` and `terraform state show`
- `git diff`
- `git show`
- `curl` and `wget` responses

Current policy is metadata only for these commands, with no full-output
opt-in in this milestone. Ordinary Docker/Compose status tables and known
Kubernetes inventory tables remain eligible for capture; raw Kubernetes object
specs, descriptions, and custom output formats do not.

---

### [x] Cap all persisted command output

Both eligible modes and direct SQLite output producers now enforce 64 KiB per
stream, including markers, and preserve original counts/truncation flags.
Existing rows are unchanged. Temporary Bash capture files remain unbounded;
that separate risk stays under temporary output storage review.

Requirements:

- Apply a hard size limit to every output capture mode.
- Preserve useful context when truncating.
- Record original byte counts.
- Preserve truncation metadata.

---

### [x] Replace head-only truncation with head + tail capture

Oversized ordinary output now retains roughly 16 KiB of head and 48 KiB of tail,
with complete boundary lines and an explicit omission marker. Oversized capture
files requiring redaction conservatively withhold the tail to avoid exposing
multiline secret fragments. Terraform plan/apply/destroy tail summaries pass
regression tests. See `docs/SECURITY-PRIVACY.md` for the privacy exception.

The previous head-only behavior could remove important forensic information, including:

- Terraform `Plan:` summaries
- Terraform `Apply complete!`
- recent log lines
- final error messages

Implemented behavior:

- preserve a useful head section,
- preserve a larger tail section,
- insert an explicit omission marker.

Terraform semantic parsing must still work on truncated output.

---

### [x] Lock down WakeTrail state permissions

WakeTrail now enforces 0700 on its own state directory and 0600 on the database,
state JSON, snapshots, and watcher log. Existing files are tightened on state
access. SQLite's active rollback journal is verified as private. New and
explicitly overwritten exports are private; arbitrary past exports are untouched.
The watcher log now respects XDG_STATE_HOME. Parent state permissions are not
changed. These paths and limitations are documented in SECURITY-PRIVACY.md.

Requirements:

- state directory: `0700`
- SQLite database: user-readable/writable only
- state JSON files: `0600`
- watch log: `0600`
- review exported report permissions

Document where WakeTrail stores local data.

---

# 2. Recording Model and Retro Safety

## Important

### [ ] Define the explicit always-on recording model

WakeTrail currently records command events outside active sessions.

Before `--retro` becomes a public feature, define exactly what happens
when no session is active.

Decide:

- Is retro buffering opt-in?
- Are command bodies stored?
- Is command output stored?
- Are Git contexts stored?
- Are collector events continuously buffered?
- What is the default retention period?
- What happens when the buffer reaches its limit?

The UI and documentation must make buffering visible.

---

### [ ] Add automatic retro-buffer pruning

Retro data must not grow indefinitely.

Requirements:

- bounded retention
- automatic pruning
- index timestamp fields needed for retro queries
- eventual manual prune/purge command

---

### [ ] Prevent accidental active-session replacement

`waketrail start` should not silently replace an already-active session.

Desired behavior:

- refuse to start,
- identify the active session,
- suggest `waketrail stop`,
- optionally support an explicit `--force` behavior later.

---

### [ ] Review multi-shell session behavior

WakeTrail currently has one global active session.

Investigate commands from multiple terminals/shells being attributed to
the same session.

Possible future identifiers:

- shell PID
- parent PID
- TTY
- working-directory scope

Decide the v1 behavior and document it.

---

# 3. Bash and Shell Integration

## Important

### [?] Verify VS Code shell integration compatibility

Observed during the collector card-gallery test:

- WakeTrail functions remained defined.
- `PROMPT_COMMAND` no longer contained `__waketrail_precmd`.
- VS Code owned the active `DEBUG` trap.
- manually sourcing the WakeTrail hook restored WakeTrail,
  but replaced the VS Code `DEBUG` trap.

WakeTrail must coexist with common shell integrations rather than relying
on "last hook loaded wins."

Test with:

- VS Code integrated terminal
- Starship
- zoxide
- existing `PROMPT_COMMAND`
- existing `DEBUG` trap

---

### [?] Verify compound commands and pipelines

Explicitly test:

```bash
cd infra && terraform apply
kubectl get pods | grep api
a && docker ps && kubectl get pods
```

Verify:

- commands are captured correctly,
- output capture starts/stops correctly,
- no orphan `tee` processes remain,
- file descriptors are restored,
- no duplicate events are produced.

---

### [?] Verify interrupted commands

Test:

- Ctrl-C during captured output
- shell exit during capture
- terminal closure
- nested Bash shells

Check for:

- orphan temporary files
- stuck capture state
- leaked file descriptors
- incorrect exit codes

---

### [x] Review heredoc command capture

WakeTrail records the shell command text. A heredoc used to create or modify a
file can therefore place the heredoc body directly into command history.

Test and document behavior for commands such as:

```bash
cat > config.yaml <<'EOF'
...
EOF
```

Implemented conservative body omission before persistence: multiline commands
retain the redacted first invocation line plus an omission marker. Heredocs and
here-strings retain the invocation prefix before `<<` plus a body-omission marker.
Body contents are not persisted. This is deliberately not a shell parser;
quoted occurrences of `<<` may also be conservatively omitted.

---

### [ ] Review temporary output storage

Prefer per-user runtime storage where possible rather than raw `/tmp`.

Investigate:

- `$XDG_RUNTIME_DIR`
- cleanup after abnormal shell termination
- secure temp-file permissions

---

### [ ] Review hook performance

The shell prompt must remain responsive.

Measure:

- command classification overhead
- recorder process startup
- SQLite open/migration cost
- Git-context collection cost

Do not optimize blindly. Measure before changing architecture.

---

# 4. Collector Reliability

## Important

### [ ] Add collector execution timeouts

A hung external command must not stall every collector.

Examples:

- unreachable Kubernetes API
- blocked Docker command
- systemd command waiting unexpectedly

Use bounded execution time for collector subprocesses.

---

### [ ] Surface collector failures and blind spots

A forensic timeline must not silently imply that nothing happened when a
collector actually failed.

Consider timeline events such as:

- `collector_error`
- `collector_gap`
- `collector_resumed`

Detached watcher errors should also be written to the diagnostic log.

---

### [ ] Retry failed collector initialization

A collector that fails its first snapshot should not necessarily be
disabled for the entire session.

Investigate retry/backoff behavior.

---

### [ ] Review sequential collector polling

Current collectors run sequentially.

Determine whether a slow collector can delay unrelated sources enough to
matter for v1.

Do not introduce concurrency unless testing shows it is necessary.

---

# 5. Terraform Hardening

## Important

### [x] Parse Terraform plan summaries

### [x] Parse Terraform apply summaries

### [x] Parse Terraform destroy summaries

### [x] Normalize replacement counts

### [x] Strip ANSI formatting before parsing

### [x] Correlate semantic Terraform events with commands

### [x] Add Terraform-specific PLAN / APPLY / DESTROY presentation

### [ ] Handle Terraform "No changes"

Recognize successful no-op output such as:

```text
No changes. Your infrastructure matches the configuration.
```

Present this as a successful result rather than a state change.

---

### [ ] Handle failed Terraform operations

A failed plan/apply/destroy should create a meaningful semantic event
instead of only appearing as a generic failed command.

Possible presentation:

```text
Terraform: APPLY FAILED
Deployment: example
Error: ...
Command: terraform apply
```

Do not persist unbounded error output.

---

### [ ] Verify partial apply behavior

Determine how WakeTrail should represent an apply that changed some
resources before failing.

---

### [ ] Verify Terraform command variants

Test:

- `terraform`
- `tofu`
- `sudo terraform`
- `terraform -chdir=...`
- relevant global flags

Classifier and semantic parsing must agree.

---

# 6. Git Collector Review

## Important

Git was implemented early in WakeTrail and should be reviewed against the
newer collector/event model before v1.

### [ ] Review Git context collection performance

Current implementation invokes multiple Git commands.

Investigate whether Git context can be collected using a single command,
for example porcelain-v2 branch/status information.

Avoid unnecessary index locking.

---

### [ ] Handle Git edge cases

Test:

- detached HEAD
- empty repository
- worktrees
- submodules
- `safe.directory` failures
- repository with no commits

---

### [x] Decide what constitutes a semantic Git event

Semantic Git upgrade is implemented and smoke-tested. Covered events include
COMMIT, BRANCH SWITCH, HEAD CHANGED, FILES CHANGED, and WORKING TREE. Edge-case
and performance verification remain under their separate checklist items.

Original review candidates:

- commit created
- branch switched
- merge completed
- rebase completed
- repository became dirty
- repository became clean
- HEAD changed

Normal presentation should avoid showing Git noise under every unrelated
command.

Raw Git context may remain available for forensic/verbose use.

---

### [x] Review changed-file reporting

WakeTrail documentation has referenced changed files.

Git snapshots retain filenames and file status metadata. Diff contents are not
stored by default; output capture now excludes diff/show and log patch flags.

---

# 7. Correlation and Timeline Integrity

## Review

### [ ] Review Docker failure/recovery correlation

Current lifecycle correlation focuses primarily on explicit start/stop
operations.

Evaluate correlation for:

- restart
- crash after command
- recovery
- multiple container effects

---

### [x] Review Docker Compose grouping

One Compose command may affect many containers.

Compose up/down grouping and project attribution from actual Compose labels
have passed live smoke tests. Activities now use the shared card fields for
terminal output and Markdown export: Deployment, Effects, Scope, Command.

Aggregate transitions cover only observed affected containers and distinct
labelled services. Services means services with affected containers present,
including stopped containers; it does not imply readiness or project inventory.
Created/removed count unique affected containers; running/healthy compare the
first and last observed state. Unchanged rows are omitted, and verbose retains
raw per-container events. A removal-only observation does not establish the
prior running/health state, so those transitions are omitted rather than guessed.

Verified against the installed CLI and live `compose-smoke-2` session in normal
and verbose views. Docker/Compose milestone committed and pushed by Jeff.
Remaining correlation edge cases stay within the separate v1 review.

---

### [ ] Review systemd failure/recovery correlation

Ensure failures and recoveries can be connected to meaningful commands
when confidence is high.

---

### [ ] Review Kubernetes command coverage

Current high-confidence grouping is strongest for `kubectl scale`.

Evaluate the v1 value of adding:

- rollout restart
- delete
- apply
- patch
- set image

Do not attempt speculative correlation merely to increase coverage.

---

### [x] Share timeline reconstruction between show and export

Both consume the shared semantic pipeline in `cmd/timeline.go` and activity data.
`card.go` owns the shared activity field grammar for terminal and Markdown.
Keeping this model in `cmd` is sufficient for v1; no package migration is required.

---

# 8. Presentation Consistency

## Important

### [x] Define one card layout contract

Shared card grammar is implemented in `cmd/card.go`, including Compose aggregate
activities. Terminal and Markdown activity fields share the same builder.
Full release-gallery verification remains its own checkpoint.

Preferred hierarchy:

```text
Collector: EVENT TYPE
Resource Type: Resource
Effect / Result
Command
```

Rules:

- collector must be immediately scannable,
- use tight spacing inside logical groups,
- blank lines only between genuinely separate groups,
- commands appear consistently,
- raw events remain available in verbose mode.

Review:

- Docker
- systemd
- Kubernetes
- Terraform
- Git
- Note
- generic Command

---

### [ ] Run full collector card-gallery test

Create one session containing:

- Note
- standalone Command + Git context
- Docker
- systemd
- Kubernetes
- Terraform

Compare all card formats side-by-side.

Do not finalize card styling from isolated collector tests alone.

---

# 9. Storage Hardening

## Important

### [ ] Enable and verify SQLite foreign keys

SQLite foreign-key constraints require explicit enforcement.

Add tests proving session deletion cascades where intended.

---

### [ ] Review schema migration strategy

Current migrations inspect columns during database open.

Before v1, evaluate numbered schema migrations using
`PRAGMA user_version`.

Goal: future WakeTrail versions should be able to safely upgrade a v1
database.

---

### [ ] Consider WAL mode

Current `busy_timeout` fixed observed recorder/watcher lock contention.

Evaluate WAL under concurrent recording/watch workloads.

Only enable it if testing shows a meaningful benefit.

---

### [ ] Add indexes needed for timeline and retro queries

Review indexes for:

- session timeline lookup
- command timestamp lookup
- retro-window queries

---

# 10. Release Documentation

## Release Blocker

### [ ] Document what WakeTrail records

README/docs must state:

- command metadata captured
- when output is captured
- where local data lives
- what retro buffering records
- what collectors observe
- what redaction does
- limitations of redaction
- how to stop/pause/purge recording

---

## Important

### [ ] Rewrite README for current WakeTrail

The README predates substantial functionality.

Update:

- current architecture
- supported collectors
- semantic activity grouping
- Terraform support
- detached watcher
- Markdown export
- `--verbose`
- retro behavior once complete
- real screenshots/output

---

### [ ] Add installation and shell-hook setup

A new user must be able to install WakeTrail and activate shell capture
without reading source code.

---

### [ ] Create user documentation

Likely `docs/` topics:

- Quick Start
- Recording Model
- Collectors
- Retro Capture
- Reports and Export
- Security / Redaction
- Troubleshooting
- Architecture

---

# 11. Release Testing

## Important

### [ ] Clean-install test

Test WakeTrail from a clean environment rather than the development repo.

Verify:

- installation
- hook activation
- session start/stop
- watcher
- collectors
- export

---

### [ ] Add regression tests for bugs discovered during development

Include at minimum:

- SQLite locking regression
- ANSI Terraform output
- shell housekeeping filtering
- Kubernetes first-to-final controller formatting
- shell integration coexistence
- output truncation
- redaction test matrix

---

### [ ] Review supported platforms

Current implementation contains Linux/Unix-specific behavior.

Document actual v1 support rather than implying unsupported platforms.

---

# 12. Explicitly Deferred Beyond v1 Hardening

These are valuable ideas but are not automatically v1 release blockers.

- Event-stream replacements for all polling collectors
- Full probabilistic/scored command attribution
- Arbitrary wrapper/script attribution
- Complete shell-language parser
- Deep Kubernetes Event ingestion
- Full Terraform backend/workspace/state intelligence
- Cloud CLI collectors
- Helm collector
- Ansible collector
- Replay mode
- Advanced filtering/search
- Comprehensive fuzzing
- Broad platform support

Items may be promoted if testing proves they are necessary for correctness.

---

# 13. External Review Checkpoint

When the active build work reaches a stable checkpoint:

- [ ] Commit all current changes
- [ ] Ask an independent reviewer to test secret redaction adversarially
- [ ] Ask an independent reviewer to torture-test Bash hook behavior
- [ ] Compare findings against this checklist
- [ ] Verify reported bugs locally before changing architecture

Do not delegate product architecture or parallel implementation while the
core design is actively changing.

---

# v1 Hardening Exit Criteria

WakeTrail is ready to leave hardening when:

- [ ] All Release Blockers are completed
- [ ] No known issue can silently expose common secrets
- [ ] Capture size is bounded
- [ ] Retro retention and privacy behavior are explicit
- [ ] Collector failures cannot silently masquerade as healthy observation
- [ ] Shell capture has been tested with common integrations
- [ ] Git collector review is complete
- [ ] Terraform success, failure, no-op, and destroy paths are tested
- [ ] Collector cards use a consistent presentation grammar
- [ ] README and user documentation match the actual product
- [ ] A clean installation passes the end-to-end demo

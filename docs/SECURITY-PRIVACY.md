# Security and Privacy

WakeTrail stores local command metadata and selected diagnostic output. New
command text, stdout/stderr, and timeline text are sanitized before SQLite
writes. Notes also receive redaction. This does not rewrite existing sessions
or previously exported reports.

## Capture policy

| Command family | Current output policy |
| --- | --- |
| Docker/Compose `ps` | Eligible for capture; existing scoped STATUS interpretation remains |
| Kubernetes `get` tables for known inventory types | Eligible; ordinary and wide tables only |
| Kubernetes secrets, config maps, custom resource types, descriptions, YAML/JSON/custom output | Metadata only |
| Docker `inspect`, Compose `config` | Metadata only |
| Terraform/OpenTofu `output`, `show`, `state show` | Metadata only |
| Git `diff`, `show`, and log patch flags | Metadata only |
| curl/wget response bodies | Metadata only, including unauthenticated requests |
| Recognized logs and Terraform plan/apply/destroy | Bounded capture with redaction |
| Other eligible diagnostics | Capture with redaction |
| Pipelines, command lists, substitutions, redirections, multiline commands | Metadata only |

curl/wget responses can contain credentials even without authentication flags.
This policy therefore does not guess response sensitivity from a URL. Command
text, exit code, duration, and working directory are still recorded.

The shell classifier decides whether to begin capture. The recorder independently
rechecks classification before reading supplied output files; requesting
`--capture-mode output` does not override a denial. There is no full-output
opt-in for denied commands in this milestone.

The Bash hook uses a fresh interactive history entry to classify the whole
submitted line, since DEBUG alone exposes pipeline/list fragments. When history
is disabled, ignored, or unchanged (including duplicates suppressed by
HISTCONTROL), output falls back to metadata only. In that fallback, command text
can reflect the DEBUG fragment rather than a full compound line. Shell history
settings are not changed. The syntax guard is intentionally conservative and
does not implement a shell parser. Shell operators inside quoted strings may
also disable capture. Commands
with unrecognized wrappers/options can fall back to metadata only. Only known
Kubernetes table resource types/options are accepted.

## Command bodies

Multiline commands retain a redacted first invocation line and
`[multiline body omitted]`. Heredocs/here-strings retain the redacted invocation
prefix before `<<`, followed by `[shell body omitted]`. Subsequent body content
is not stored. A quoted `<<` can also trigger conservative omission.

WakeTrail does not change the command you execute or your shell's own history.
It sanitizes its stored representation.

## Redaction

Supported patterns include sensitive variable/key suffixes, quoted JSON/YAML
values and nested sensitive flow values, YAML multiline blocks, common password/token flags, URL credentials,
Authorization/Cookie headers, AWS access keys, known GitHub/GitLab/Slack token
prefixes, JWT-like strings, and private-key PEM blocks. Escaped and concatenated
shell values are covered for recognized sensitive assignments/flags.

Redaction is **best-effort, not a security guarantee**. Arbitrary unlabelled
strings, encoded values, novel token formats, and unusual syntax can evade
pattern matching. Eligible logs and diagnostics can still contain sensitive
content. Capture restrictions reduce exposure rather than relying on regexes
to make intentionally secret-bearing output safe.

Original output byte counts describe the observed bytes before redaction, so
stored text length may differ. The updated Bash hook forwards live output through
an internal streaming helper. Raw bytes stay in bounded process memory; only the
sanitized capture and a small metadata header are written to temporary disk.
Normal prompt/EXIT cleanup removes the command's private temporary directory.

## Output limits

Every eligible output mode retains at most **64 KiB per stream** in new SQLite
rows, including omission markers. Stdout and stderr have independent limits.
The recorder and SQLite write boundary both enforce the limit; existing rows
are not rewritten.

Oversized ordinary diagnostics keep roughly **16 KiB of head and 48 KiB of tail**.
Only complete boundary lines are retained, so a single oversized line may be
omitted entirely. An explicit marker identifies the missing section; original
byte counts and truncation flags remain available. Terraform plan/apply/destroy
summaries near the end remain parseable. Truncated status tables are still
excluded from inventory interpretation.

Redaction context takes precedence over retaining a tail. Bounded-memory
overlapping windows inspect the stream while it is forwarded. Legacy plain
capture files receive the same inspection in the recorder. If an oversized
stream requires redaction, only the sanitized head is retained, with a marker
explaining that the tail was withheld. This conservatively covers multiline
private keys
and YAML/JSON credentials whose headers may lie in the omitted middle. Small
streams are sanitized in full. In-memory producers are sanitized before their
head/tail sections are selected. Pattern detection is still best-effort.

The limit bounds persisted output and capture/recorder working buffers. Each new
stdout/stderr spool is capped at **65,566 bytes**: at most 64 KiB of sanitized text
plus its 30-byte header. The spool is finalized at EOF; while a command is running,
its sample stays in bounded memory. The helper still processes every live byte,
so forwarding/redaction work scales with output size.

Install the matching CLI before loading the updated hook. Then re-source
`shell/bash/waketrail-hook.sh` or start a new configured shell. Already-loaded
older hooks continue to use plain `tee` files until reloaded; the updated recorder
can still read those files.

## Local recording and storage

The default data location is `~/.local/state/waketrail/`, including
`waketrail.db`. Setting `XDG_STATE_HOME` moves it to
`$XDG_STATE_HOME/waketrail/`. Markdown exports use the requested output path,
or a session-derived filename in the current directory.

The current hook can record commands outside a named active session. The
always-on/retro retention policy remains a separate v1 hardening item.
`waketrail stop` ends the named session; it does not disable the shell hook.
To stop shell capture, disable hook sourcing in your shell configuration and
start a new shell. Stop a detached watcher with `waketrail watch stop`.

WakeTrail's own state directory uses `0700`. The SQLite database, its existing
sidecars, session/watch/snapshot JSON files, and watcher log use `0600`. Opening
state repairs these permissions on older installations without rewriting their
contents or changing the parent state directory's permissions. Detached watcher
logs follow `XDG_STATE_HOME` too. New and overwritten exports use `0600`;
previous exports are only changed when explicitly overwritten. Export and state
file writes reject symlink destinations.

An explicit retention/purge workflow and abnormal shell cleanup remain tracked
in `V1-HARDENING.md`.

## Temporary storage and shell cleanup

The updated hook creates a private `0700` directory per command. It prefers
`XDG_RUNTIME_DIR` when that directory is owned by the current user, writable, and
has mode `0700`. Otherwise it uses `TMPDIR`, falling back to `/tmp`, via `mktemp -d`.
Output spools and optional Git metadata snapshots use `0600`. No shared parent
directory permissions are changed. Git snapshots retain metadata and filenames;
the stdout/stderr spool limit does not redefine Git snapshot contents.

At the next prompt, the hook restores stdout/stderr, waits for the stream helpers,
records the result, and removes the temporary files/directory. Helpers survive
Ctrl-C long enough to finalize output already observed; interrupted commands keep
their exit code (normally 130). EXIT cleanup restores descriptors and removes
capture files while preserving a previously installed EXIT handler and its
original status. A pseudo-terminal hangup test also verifies cleanup through EXIT.
Exiting before the next prompt cleans temporary data but does not create a final
command row.

SIGKILL and host crashes cannot run EXIT cleanup and may leave a private bounded
spool/directory. There is no stale-directory sweeper in this slice. Background
processes that inherit capture descriptors can delay EOF and the prompt's wait;
background activity attribution and broader shell integration remain separate
compatibility work. Fresh history makes compound lines metadata-only; repeated
DEBUG events restore prior descriptors before replacing capture handles,
preventing stacked capture files. This does not reconstruct per-command effects
inside a compound line.

# WakeTrail

[![CI](https://github.com/it-odyssey/waketrail/actions/workflows/ci.yml/badge.svg)](https://github.com/it-odyssey/waketrail/actions/workflows/ci.yml)

**A local forensic timeline for DevOps work.**

**Record the wake your changes leave behind.**

WakeTrail records and correlates engineering activity with the state changes that follow it, creating a chronological record of what happened during a troubleshooting, deployment, or infrastructure session.

The goal is simple:

> When something breaks, you should be able to reconstruct what you did, what changed, and what happened next.

## Why WakeTrail?

Traditional shell history can tell you which commands were entered.

Terminal recorders can show you what appeared on the screen.

Observability platforms can tell you what a running system is doing.

WakeTrail is intended to connect those perspectives.

A WakeTrail session can eventually correlate:

- commands executed
- timestamps
- working directories
- exit codes
- execution duration
- Git repository, branch, and commit state
- changed files
- Docker and container health
- systemd service state
- infrastructure changes
- Kubernetes state transitions
- Terraform / OpenTofu operations
- relevant logs and journal events
- manually marked incidents
- recovery events

The result is a forensic timeline of engineering work rather than simply a command history.

## Example

A future WakeTrail timeline might look like:

```text
14:31:08  SESSION START
           homelab-rebuild

14:32:14  $ terraform apply
           exit: 0
           duration: 18.2s

           git:
             branch: tailscale-rebuild
             commit: 85bc21a
             dirty: yes

           terraform:
             +3 created
             ~2 changed

14:34:03  $ docker compose up -d
           exit: 0

14:34:09  SERVICE STATE CHANGE
           traefik: healthy → restarting

14:34:24  SERVICE STATE CHANGE
           traefik: restarting → unhealthy

14:35:02  INCIDENT
           "Traefik stopped responding"

14:37:41  $ docker compose restart traefik
           exit: 0

14:37:48  SERVICE RECOVERED
           traefik: unhealthy → healthy
```

The commands are important, but the changes they caused are the real story.

## What WakeTrail Is Not

WakeTrail is not intended to replace:

- shell-history tools such as Atuin
- terminal-session recorders
- distributed tracing systems
- log aggregation platforms
- general-purpose observability stacks

WakeTrail focuses specifically on reconstructing the timeline of engineering actions and their effects on a local or managed environment.

## Current Status

WakeTrail is in early development.

The current CLI supports:

```bash
waketrail start <session-name>
waketrail status
waketrail observe
waketrail mark
waketrail stop
waketrail show
waketrail list
waketrail help
waketrail watch <service>
```

The Bash integration can detect interactive commands and capture their exit status.

WakeTrail can selectively capture output from diagnostic commands such as
`pwd`, `git status`, `docker ps`, `docker compose ps`, and ordinary Kubernetes
inventory tables. Secret-prone inspections and arbitrary curl/wget response
bodies retain command metadata without output capture.

Potentially large output sources such as docker logs, journalctl, kubectl logs, and Terraform plans use bounded capture to avoid flooding the local event store. Session timelines display concise output previews while preserving the underlying captured evidence.

New command text, captured output, and timeline text receive best-effort redaction
before SQLite writes. Multiline command bodies are omitted. Redaction cannot
recognize every secret and does not sanitize existing recordings. See
[Security and Privacy](docs/SECURITY-PRIVACY.md) for the exact capture policy,
recording behavior, storage location, and limitations.

WakeTrail is continuing to expand environment correlation, automated state monitoring, redaction, and report generation.

## Design Principles

WakeTrail is being built around a few core principles:

- **Local first** — no cloud account or external service required
- **Low friction** — record work without changing normal command-line habits
- **Forensic context** — capture what changed, not just what was typed
- **Chronological correlation** — reconstruct cause, effect, failure, and recovery
- **Modular architecture** — integrations remain independent of the recorder core
- **Open source** — built for real Linux and DevOps workflows
- **Omarchy native** — provide a native Omarchy interface while remaining useful outside Omarchy

## Planned Architecture

```text
WakeTrail
│
├── CLI / recorder core
│
├── shell integrations
│   └── Bash
│
├── local event store
│
├── timeline engine
│
├── collectors
│   ├── Git
│   ├── Docker / Compose
│   ├── systemd
│   ├── Kubernetes
│   ├── Terraform / OpenTofu
│   └── logs / journal
│
├── incident capture
│   ├── markers
│   ├── snapshots
│   └── exports
│
└── Omarchy plugin
    ├── recording indicator
    ├── session controls
    ├── recent events
    ├── failures
    └── session timeline
```

## Development

WakeTrail is written in Go.

Run locally:

```bash
go run .
```

Run all tests:

```bash
go test ./...
```

## Roadmap

Initial milestones:

- [x] CLI foundation
- [x] Session start / status / stop
- [x] Bash command capture prototype
- [x] Persistent command events
- [x] Execution duration tracking
- [x] Git context capture
- [x] SQLite event storage
- [x] Session timeline output
- [x] Generic timeline event model
- [x] Manual timeline notes
- [x] Selective command output capture
- [x] Bounded stdout/stderr capture
- [x] Command output previews in session timeline
- [x] Secret and sensitive-data redaction
- [x] Continuous watch mode
- [x] Structured resource-aware timeline events
- [x] Command-to-effect correlation
- [x] Activity grouping / semantic presentation
- [x] Detached watch mode — waketrail watch -d
- [x] Session listing / history
- [x] Docker / Compose collector
- [x] systemd collector
- [x] Terraform / OpenTofu collector
- [x] Kubernetes collector
- [x] Git collector
- [x] Automatic state-change detection
- [x] Automatic failure and recovery events
- [ ] Rolling retroactive buffer with `--retro`
- [ ] Omarchy plugin
- [ ] Incident markers
- [ ] Incident snapshot / export
- [ ] Collector-specific activity classification
- [ ] Additional shell support
- [ ] Window-only screenshot capture
  - [ ] Screenshot capture will be window-only and opt-in.
- [ ] Evidence and artifact attachments
- [ ] Report generation
  - [x] Markdown session export
  - [ ] Incident report
  - [ ] Runbook
  - [ ] Portfolio case study
  - [ ] Work receipt
  - [ ] Change record
  - [ ] troubleshooting notes
  - [ ] customer handoff
- [ ] Optional AI-assisted report synthesis
- [ ] Revisit licensing and commercial model before public beta or contributions

## IT Odyssey

WakeTrail is an open-source project from **IT Odyssey**.

**Embrace The Journey.**

## License

WakeTrail is licensed under the [Apache License 2.0](LICENSE).

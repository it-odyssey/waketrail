# Timeline/card refactor: validation checklist

This change is display-time only: no SQLite migration and no changes to existing stored events.

## What changed

- `cmd/show.go` handles CLI session loading only; `cmd/timeline.go` constructs the shared display timeline for both `show` and `export`.
- `cmd/correlation.go` handles command attribution and collector lifecycle command matching.
- `cmd/report.go` renders report headers and standalone shell commands.
- `cmd/card.go` renders all collector cards, including grouped Kubernetes effects. Notes intentionally remain a distinct compact card.
- Namespace-scoped successful `kubectl apply` now produces a grouped Kubernetes APPLY activity from events observed in a limited window. Attribution is time-and-namespace based; it is **not** a verified Kubernetes owner-reference causal chain.
- Normal report classification avoids calling an initially Running-but-not-Ready pod an immediate failure if it matches the known startup transition. Recorded events remain unchanged.
- Old Docker `state_change` appearance/removal rows get CREATED/REMOVED display labels without rewriting their stored type.

## Validate on Omarchy

```bash
gofmt -l cmd
go vet ./...
go test ./...
go install .
waketrail show card-gallery-2
waketrail show boutique-install
waketrail export boutique-install -o /tmp/boutique-install-refactored.md
```

For a new Docker lifecycle test, **start the collector before the session** and allow the initial snapshot to establish its baseline:

```bash
waketrail watch -d
waketrail watch status
# Wait for collector baseline, then:
waketrail start docker-refactor-check
docker run -d --name wt-refactor-test nginx:alpine
docker stop wt-refactor-test
docker start wt-refactor-test
docker rm -f wt-refactor-test
waketrail stop
waketrail show docker-refactor-check
waketrail watch stop
```

The collector is polling. Brief Docker transitions can be missed between polls, especially if commands are executed back-to-back. If no Docker timeline events were stored, neither `show` nor `export` can reconstruct them from commands alone.

## What is NOT done

- No new secret-redaction or capture-allowlist guarantees: the relevant v1 security hardening is still required before a public release.
- Docker Compose project grouping and Docker/Compose `ps` status snapshots are not implemented.
- Kubernetes activity attribution is heuristic; it excludes unrelated namespaces but may still group concurrent changes within a namespace.
- The original raw events and historical failure/recovery classifications remain preserved. Summary metrics may differ when normalization is applied at display time.
- No new background watch-start behavior is added when recording a session.

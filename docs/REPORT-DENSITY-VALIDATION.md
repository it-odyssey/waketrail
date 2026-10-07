# WakeTrail report-density pass

## Scope

Drop-in replacements: `cmd/activity.go`, `cmd/card.go`, `cmd/report.go`, `cmd/export.go`; adds `cmd/density_test.go`. No SQLite migration or collector changes.

- Kubernetes namespace-scoped APPLY activity shows **observed** controller/pod readiness counts, observation interval, and status rather than listing every resource. This reports data observed within WakeTrail's bounded grouping window; it is **not** a cluster-wide inventory, and the apply command's success is not proof of readiness.
- Kubernetes transitions from `Running ready 0/n` to `Running ready n/n`, and a normal deployment ready/available 0->1 transition, are considered state progression for display/summary purposes, not recovered incidents. Raw rows are unchanged. This is a limited heuristic, not a comprehensive incident classifier.
- `show --verbose` now prints the **entire stored** stdout and stderr, with a separate warning when capture-time truncation occurred. The default preview is three lines/240 characters.
- Common card and NOTE/header box widths now use the same width setting. Long structured card values are shortened with a middle ellipsis, while command fields wrap without dropping content.
- Markdown exports share the compact Kubernetes APPLY summary and existing event reclassification. Existing capture remains untouched.
- Exported Markdown files are newly created with 0600 permissions. Existing overwritten file permissions may be preserved by the filesystem; this does not yet remediate state-directory/database permissions.

## Verify on Omarchy

From the repo root, apply contents of the ZIP preserving paths, then run:

```bash
gofmt -l cmd
go vet ./...
go test ./...
go install .
waketrail show card-gallery-2
waketrail show boutique-install
waketrail show boutique-install --verbose
waketrail export boutique-install --output /tmp/boutique-density.md
```

Note: `show --verbose` reveals all *stored* output, not data discarded by the original capture. Inspect sensitive content before sharing exported reports.

## Review checkpoints

1. Compare summary and NOTE box widths to collector cards.
2. Verify the APPLY status only claims observed readiness and that no unexpected resources were attributed to the namespace.
3. Verify all long resource-name lines stay inside cards; the full names remain in SQLite and verbose/raw output.
4. Confirm the 7/19 startup failure/recovery counts are reduced appropriately for this specific rollout; do not expect arbitrary Kubernetes incidents to be reclassified.
5. In verbose mode, verify `kubectl get pods --watch` prints all recorded output and any capture truncation warning.
6. Confirm Markdown export still renders and uses the same activity counts.

## Still outstanding

The collector currently polls, and a Kubernetes APPLY activity is correlated by namespace/time, not manifest or owner UID. That can over-attribute unrelated changes in the same namespace. Captured command content may be secret-bearing; future hardening must improve redaction, capture size limits and state-directory permissions. A per-activity `--details` mode and a structured export schema are not part of this pass.

## Test environment limitation

The archive was parsed and formatted with `gofmt`. Full package tests could not be run in the packaging environment because it has Go 1.23.2 while `go.mod` requests Go 1.27.0. Run the commands above before installing the binary.

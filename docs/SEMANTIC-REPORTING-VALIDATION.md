# Semantic Reporting Stabilization — Validation

This is a coordinated replacement for the current report-density commit. It introduces **no database migration** and preserves stored command output and timeline records.

## Scope

- `kubectl get pods`/`-o wide` completed inspections display a semantic `Kubernetes: STATUS` card in normal mode **only when all stored output is available and parseable**. The same interpretation is used for Markdown export. `--verbose` retains full stored output. A watched stream is not a snapshot, nor are failed commands or truncated captures.
- Readiness lag alone is not a Kubernetes failure. Older timeline events are **interpreted** with the corrected rules without overwriting SQLite. Recognized pod error reasons and node health incidents remain meaningful. Kubernetes collector now treats controller convergence as state change rather than declaring an incident from replica deficit alone. DaemonSet misscheduling remains actionable.
- APPLY summaries state **observed** readiness and explicitly label attribution as namespace/time correlation; they do not prove object ownership or completeness. An incomplete observed rollout does not receive a green success symbol.
- Summary header extends to the collector cards' right edge. NOTE stays in the collector gutter. Resource-name ellipses only affect presentation; long command fields wrap.
- Terminal and Markdown reporting reuse the same normalized incident and observed status interpretation.

## Limitations worth keeping explicit

1. `kubectl get pods` status parser supports human-readable conventional and wide tables, not `-o json`, `-A`, custom columns, watched streams, or headerless output. Unsupported content stays a plain command. An empty `No resources found` response is not incorrectly called healthy.
2. Pod errors such as `CrashLoopBackOff`, `ImagePullBackOff`, `OOMKilled`, and failed phases are incidents. Mere `Running ready 0/1` is not. Native Kubernetes Events and restart history were not recorded in prior sessions and cannot be reconstructed.
3. Historical controller readiness decreases alone are represented as state transitions rather than verified failures; separate pod or node evidence is required to establish the root cause. A later dedicated rollout-health tracker could add timeouts and explicit degraded-state semantics.
4. Attribution from command proximity and namespace is provisional. Cross-process concurrent changes in the same namespace cannot be proven to belong to the apply without manifest/owner UID information.
5. This update deliberately does not implement Docker Compose, retro buffer, Bash coexistence or release-security hardening.

## Required Omarchy validation

From the repository root **before** running `go install .`:

```bash
git status
gofmt -l cmd internal/collectors/kubernetes
go vet ./...
go test ./...
go build ./...
```

Then:

```bash
go install .
waketrail show boutique-install
waketrail show boutique-install --verbose
waketrail show card-gallery-2
waketrail export boutique-install --output /tmp/boutique-semantic-report.md
```

**Visual expectations:** the Boutique summary no longer counts ordinary startup readiness lag as seven failures; normal mode displays a `Kubernetes: STATUS` snapshot for `kubectl get pods -o wide`; `kubectl get pods --watch` remains an ordinary failed command; verbose preserves the full stored output, including capture-truncation warnings; the summary header right edge aligns with collector cards; completed and incomplete APPLY observations have distinct visual outcomes.

Do **not** commit or install if tests fail; capture the exact error first. This archive has source-formatting and syntax validation, **not a full Go test run**: the build environment used for packaging is Go 1.23, while the project specifies Go 1.27.

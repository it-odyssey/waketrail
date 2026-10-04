# WakeTrail Session: redaction-test-2

## Summary

- Started: 2026-10-03 22:08:52
- Ended: 2026-10-03 22:09:42
- Duration: 49s
- Events: 3
- Failures: 0
- Recoveries: 0

## Timeline

### 22:08:52 — COMMAND

```text
waketrail start redaction-test-2
```

- Exit code: 0
- Duration: 15ms
- Git: main@73f05f7 (dirty)

### 22:09:04 — COMMAND

```text
echo 'TOKEN=[REDACTED]' > /tmp/waketrail-redact-test
```

- Exit code: 0
- Duration: 2ms
- Git: main@73f05f7 (dirty)

### 22:09:12 — COMMAND

```text
grep TOKEN /tmp/waketrail-redact-test
```

- Exit code: 0
- Duration: 12ms
- Git: main@73f05f7 (dirty)

**stdout**

```text
TOKEN=[REDACTED]
```


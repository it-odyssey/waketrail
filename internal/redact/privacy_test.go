package redact

import (
	"strings"
	"testing"
)

func TestPrivacyFormats(t *testing.T) {
	cases := []struct{ name, input, secret string }{
		{"concatenated shell word", `TOKEN="synthetic-"'concatenated-secret'`, "concatenated-secret"},
		{"escaped shell space", `PASSWORD=synthetic\ escaped-secret`, "escaped-secret"},
		{"nested JSON credentials", `{"credentials":{"username":"synthetic-nested-user","values":["synthetic-nested-secret"]},"port":5432}`, "synthetic-nested-secret"},
		{"YAML folded value", "DB_PASSWORD: >\n  synthetic-folded-value\nport: 5432", "synthetic-folded-value"},
		{"YAML implicit block", "credentials:\n  value: synthetic-implicit-secret\nport: 5432", "synthetic-implicit-secret"},
		{"prefixed env", "GITHUB_TOKEN=synthetic-env-token", "synthetic-env-token"},
		{"session env", "AWS_SESSION_TOKEN=synthetic-session-token", "synthetic-session-token"},
		{"database env", "POSTGRES_PASSWORD='synthetic password with spaces'", "synthetic password"},
		{"quoted JSON", `{"password": "synthetic-json-password", "port": 5432}`, "synthetic-json-password"},
		{"escaped JSON", `{"apiKey": "synthetic\"escaped-key"}`, "escaped-key"},
		{"quoted YAML", "DB_PASSWORD: \"synthetic yaml password\"", "synthetic yaml password"},
		{"camel case", `{"accessToken": "synthetic-access-token"}`, "synthetic-access-token"},
		{"equals flag", "tool --access-token=synthetic-flag-token", "synthetic-flag-token"},
		{"quoted flag", "tool --password 'synthetic spaced password'", "synthetic spaced password"},
		{"URL userinfo", "postgres://user:synthetic-url-password@localhost/db", "synthetic-url-password"},
		{"attached curl user", `curl -u'user:synthetic attached password' https://example.test`, "synthetic attached password"},
		{"curl user", "curl -u 'user:synthetic-basic-password' https://example.test", "synthetic-basic-password"},
		{"basic header", "Authorization: Basic c3ludGhldGljLWNyZWRlbnRpYWw=", "c3ludGhldGljLWNyZWRlbnRpYWw="},
		{"other header scheme", "Authorization: Token synthetic-other-header", "synthetic-other-header"},
		{"JSON header", `{"Authorization": "Bearer synthetic-json-header"}`, "synthetic-json-header"},
		{"cookie header", "curl -H 'Cookie: session=synthetic-cookie; user=example' https://example.test", "synthetic-cookie"},
		{"GitHub token", "ghp_abcdefghijklmnopqrstuvwxyz1234567890", "ghp_abcdefghijklmnopqrstuvwxyz1234567890"},
		{"GitLab token", "glpat-abcdefghijklmnopqrstuvwxyz123456", "glpat-abcdefghijklmnopqrstuvwxyz123456"},
		{"JWT", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.c2lnbmF0dXJl", "eyJzdWIiOiIxMjMifQ"},
		{"PEM", "-----BEGIN RSA PRIVATE KEY-----\nsynthetic-private-key-body\n-----END RSA PRIVATE KEY-----", "synthetic-private-key-body"},
		{"truncated PEM", "-----BEGIN PRIVATE KEY-----\nsynthetic-truncated-key", "synthetic-truncated-key"},
		{"unterminated quote", "PASSWORD=\"synthetic-unclosed-password", "synthetic-unclosed-password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := String(tc.input)
			if strings.Contains(got, tc.secret) || !strings.Contains(got, marker) {
				t.Fatalf("secret survived: %q", got)
			}
			if twice := String(got); twice != got {
				t.Fatalf("redaction is not idempotent: %q -> %q", got, twice)
			}
		})
	}
}

func TestYAMLBlockKeepsSiblingFields(t *testing.T) {
	input := "config:\n  password: |-\n    synthetic-block-password\n    second-secret-line\n  port: 5432\n"
	want := "config:\n  password: [REDACTED]\n  port: 5432\n"
	if got := String(input); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestOrdinaryEvidenceSurvives(t *testing.T) {
	for _, input := range []string{
		"docker compose -p monitoring ps -a", "kubectl get pods -n default",
		"Plan: 2 to add, 1 to change, 0 to destroy.",
		`{"token_count": 12, "secret_name": "database", "port": 5432}`,
		"default/web: running -> exited (0) [compose:demo/web]",
		"https://localhost:8080/health", "password policy is configured",
	} {
		if got := String(input); got != input {
			t.Errorf("altered ordinary evidence: %q -> %q", input, got)
		}
	}
}

func TestCommandBodiesAreOmitted(t *testing.T) {
	cases := []struct{ input, want string }{
		{"cat > config.yaml <<'EOF'\nunlabelled synthetic body\nEOF", "cat > config.yaml [shell body omitted]"},
		{"cat <<< 'unlabelled synthetic inline body'", "cat [shell body omitted]"},
		{"printf 'first'\nprintf 'unlabelled synthetic next line'", "printf 'first'\n[multiline body omitted]"},
		{"export PASSWORD=\"synthetic-first-line-secret\nsecond line\"", "export PASSWORD=\"[REDACTED]\"\n[multiline body omitted]"},
	}
	for _, tc := range cases {
		if got := Command(tc.input); got != tc.want {
			t.Errorf("Command() = %q, want %q", got, tc.want)
		}
	}
}

func TestSensitiveAssignmentKeepsOuterCommandQuote(t *testing.T) {
	cases := map[string]string{
		`waketrail mark "DB_PASSWORD=synthetic-note"`: `waketrail mark "DB_PASSWORD=[REDACTED]"`,
		`waketrail mark 'DB_PASSWORD=synthetic-note'`: `waketrail mark 'DB_PASSWORD=[REDACTED]'`,
		`tool "TOKEN='synthetic-value'"`:              `tool "TOKEN='[REDACTED]'"`,
	}
	for input, want := range cases {
		if got := String(input); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

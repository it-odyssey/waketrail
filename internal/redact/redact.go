package redact

import "regexp"

type rule struct {
	pattern     *regexp.Regexp
	replacement string
}

var rules = []rule{
	{
		pattern: regexp.MustCompile(
			`(?i)\b(password|passwd|token|api[_-]?key|secret|client[_-]?secret)\b\s*[:=]\s*([^\s"'` + "`" + `]+)`,
		),
		replacement: "${1}=[REDACTED]",
	},
	{
		pattern: regexp.MustCompile(
			`(?i)(--(?:password|passwd|token|api-key|apikey|secret|client-secret)\s+)([^\s]+)`,
		),
		replacement: "${1}[REDACTED]",
	},
	{
		pattern: regexp.MustCompile(
			`(?i)(authorization\s*:\s*bearer\s+)([^\s]+)`,
		),
		replacement: "${1}[REDACTED]",
	},
	{
		pattern: regexp.MustCompile(
			`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`,
		),
		replacement: "[REDACTED]",
	},
	{
		pattern: regexp.MustCompile(
			`(?i)(aws_secret_access_key\s*[:=]\s*)([A-Za-z0-9/+=]{20,})`,
		),
		replacement: "${1}[REDACTED]",
	},
}

func String(value string) string {
	redacted := value

	for _, rule := range rules {
		redacted = rule.pattern.ReplaceAllString(
			redacted,
			rule.replacement,
		)
	}

	return redacted
}

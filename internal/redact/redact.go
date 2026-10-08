package redact

import (
	"regexp"
	"strings"
)

const marker = "[REDACTED]"

// Sensitive suffixes cover names such as GITHUB_TOKEN and POSTGRES_PASSWORD
// without matching unrelated identifiers such as token_count or secret_name.
const sensitiveName = `(?:[a-z_][a-z0-9_.-]*[_-])?(?:password|passwd|token|secret|api[_-]?key|private[_-]?key|access[_-]?key|access[_-]?token|refresh[_-]?token|client[_-]?secret|secret[_-]?access[_-]?key|session[_-]?token|authorization|credentials?)`
const valuePattern = `\[REDACTED\]|(?:"(?:[^"\\]|\\.)*(?:"|\z)|'(?:[^'\\]|\\.)*(?:'|\z)|\\[\s\S]|[^\s,;}\]"'` + "`" + `\\])+`

var (
	privateKey       = regexp.MustCompile(`(?s)-----BEGIN (?:[A-Z0-9]+ )*PRIVATE KEY-----.*?(?:-----END (?:[A-Z0-9]+ )*PRIVATE KEY-----|\z)`)
	authorization    = regexp.MustCompile(`(?im)(\bauthorization["']?[ \t]*:[ \t]*["']?)([^\r\n"']+)`)
	cookie           = regexp.MustCompile(`(?im)(\b(?:set-cookie|cookie)["']?[ \t]*:[ \t]*["']?)([^\r\n"']+)`)
	assignments      = regexp.MustCompile(`(?i)(\b` + sensitiveName + `\b["']?[ \t]*[:=][ \t]*)(` + valuePattern + `)`)
	flags            = regexp.MustCompile(`(?i)(--` + sensitiveName + `(?:=|[ \t]+))(` + valuePattern + `)`)
	userFlag         = regexp.MustCompile(`(?i)((?:--user|--proxy-user|-u|-U)(?:=|[ \t]+))(` + valuePattern + `)`)
	attachedUserFlag = regexp.MustCompile(`((?:^|[ \t])-[uU])(` + valuePattern + `)`)
	urlCredentials   = regexp.MustCompile(`(?i)(\b[a-z][a-z0-9+.-]*://)[^\s/@"']+@`)
	knownToken       = regexp.MustCompile(`\b(?:AKIA[A-Z0-9]{16}|ASIA[A-Z0-9]{16}|gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,}|glpat-[A-Za-z0-9_-]{20,}|xox[baprs]-[A-Za-z0-9-]{10,})\b`)
	jwt              = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)
	structuredValue  = regexp.MustCompile(`(?i)(\b` + sensitiveName + `\b["']?[ \t]*[:=]\s*)([\{\[])`)
	yamlBlock        = regexp.MustCompile(`(?i)^([ \t]*)(["']?` + sensitiveName + `["']?[ \t]*:[ \t]*)(?:[|>][+-]?)?[ \t]*(?:#.*)?$`)
)

// String is best-effort redaction, applied before persistence. It cannot infer
// whether arbitrary unlabelled text, encoded data, or custom formats are secret.
func String(value string) string {
	value = privateKey.ReplaceAllString(value, marker)
	value = redactStructuredValues(value)
	value = redactYAMLBlocks(value)
	// Handle header schemes before generic key/value rules, which would otherwise
	// replace only the word "Bearer" and leave its credential behind.
	value = authorization.ReplaceAllStringFunc(value, func(match string) string {
		groups := authorization.FindStringSubmatch(match)
		fields := strings.Fields(groups[2])
		if len(fields) > 0 && (strings.EqualFold(fields[0], "bearer") || strings.EqualFold(fields[0], "basic")) {
			return groups[1] + fields[0] + " " + marker
		}
		return groups[1] + marker
	})
	value = cookie.ReplaceAllString(value, "${1}"+marker)
	value = redactValues(assignments, value)
	value = redactValues(flags, value)
	value = redactValues(userFlag, value)
	value = redactValues(attachedUserFlag, value)
	value = urlCredentials.ReplaceAllString(value, "${1}"+marker+"@")
	value = knownToken.ReplaceAllString(value, marker)
	return jwt.ReplaceAllString(value, marker)
}

func redactValues(pattern *regexp.Regexp, value string) string {
	return pattern.ReplaceAllStringFunc(value, func(match string) string {
		groups := pattern.FindStringSubmatch(match)
		// Preserve a scheme already sanitized by the header rule.
		if strings.Contains(strings.ToLower(groups[1]), "authorization") && (strings.EqualFold(groups[2], "bearer") || strings.EqualFold(groups[2], "basic")) {
			return match
		}
		replacement := marker
		if len(groups[2]) >= 2 && (groups[2][0] == '\'' || groups[2][0] == '"') {
			replacement = string(groups[2][0]) + marker + string(groups[2][0])
		}
		// An assignment may occur inside an outer command quote, as in
		// mark "PASSWORD=value". Its closing quote belongs to the invocation,
		// not the secret; keep it when the value has an unmatched trailing quote.
		last := groups[2][len(groups[2])-1]
		if (last == '\'' || last == '"') && unescapedQuoteCount(groups[2], last)%2 == 1 {
			replacement += string(last)
		}
		return groups[1] + replacement
	})
}

func unescapedQuoteCount(value string, quote byte) int {
	count := 0
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' {
			i++
			continue
		}
		if value[i] == quote {
			count++
		}
	}
	return count
}

// Remove a sensitive JSON/YAML flow value as one unit, keeping surrounding
// evidence. An incomplete value conservatively consumes the remaining text.
func redactStructuredValues(value string) string {
	matches := structuredValue.FindAllStringSubmatchIndex(value, -1)
	var out strings.Builder
	cursor := 0
	for _, match := range matches {
		start := match[4]
		if start < cursor || strings.HasPrefix(value[start:], marker) {
			continue
		}
		depth := 0
		quote := byte(0)
		escaped := false
		end := len(value)
		for i := start; i < len(value); i++ {
			ch := value[i]
			if quote != 0 {
				if escaped {
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
					continue
				}
				if ch == quote {
					quote = 0
				}
				continue
			}
			if ch == '"' || ch == '\'' {
				quote = ch
				continue
			}
			if ch == '{' || ch == '[' {
				depth++
			}
			if ch == '}' || ch == ']' {
				depth--
				if depth == 0 {
					end = i + 1
					break
				}
			}
		}
		out.WriteString(value[cursor:start])
		out.WriteString(marker)
		cursor = end
	}
	out.WriteString(value[cursor:])
	return out.String()
}

// YAML block indentation determines where the sensitive value ends. Keep
// sibling keys instead of consuming the remainder of a configuration document.
func redactYAMLBlocks(value string) string {
	lines := strings.Split(value, "\n")
	var result []string
	for i := 0; i < len(lines); i++ {
		groups := yamlBlock.FindStringSubmatch(strings.TrimSuffix(lines[i], "\r"))
		if groups == nil {
			result = append(result, lines[i])
			continue
		}
		result = append(result, groups[1]+groups[2]+marker)
		indent := len(groups[1])
		for i+1 < len(lines) {
			next := lines[i+1]
			if strings.TrimSpace(next) != "" && len(next)-len(strings.TrimLeft(next, " \t")) <= indent {
				break
			}
			i++
		}
	}
	return strings.Join(result, "\n")
}

// Command omits multiline bodies rather than guessing which heredoc or shell
// script fragments contain secrets. The first invocation line remains useful.
func Command(command string) string {
	first, _, _ := strings.Cut(String(command), "\n")
	multiline := strings.Contains(command, "\n")
	// Here-strings may carry unlabelled credentials entirely on the first line.
	if i := strings.Index(first, "<<"); i >= 0 {
		return String(strings.TrimSpace(first[:i])) + " [shell body omitted]"
	}
	if multiline {
		return String(first) + "\n[multiline body omitted]"
	}
	return String(command)
}

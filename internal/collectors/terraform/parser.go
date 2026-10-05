package terraform

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ChangeSummary struct {
	Add     int
	Change  int
	Destroy int
	Replace int
}

var (
	ansiPattern = regexp.MustCompile(
		`\x1b\[[0-9;]*[A-Za-z]`,
	)

	planSummaryPattern = regexp.MustCompile(
		`Plan:\s+(\d+)\s+to add,\s+(\d+)\s+to change,\s+(\d+)\s+to destroy`,
	)

	applySummaryPattern = regexp.MustCompile(
		`Apply complete!\s+Resources:\s+(\d+)\s+added,\s+(\d+)\s+changed,\s+(\d+)\s+destroyed`,
	)

	destroySummaryPattern = regexp.MustCompile(
		`Destroy complete!\s+Resources:\s+(\d+)\s+destroyed`,
	)
)

func ParsePlan(output string) (ChangeSummary, error) {
	output = stripANSI(output)

	matches := planSummaryPattern.FindStringSubmatch(
		output,
	)

	if len(matches) != 4 {
		return ChangeSummary{}, fmt.Errorf(
			"terraform plan summary not found",
		)
	}

	add, err := parseCount(matches[1])
	if err != nil {
		return ChangeSummary{}, err
	}

	change, err := parseCount(matches[2])
	if err != nil {
		return ChangeSummary{}, err
	}

	destroy, err := parseCount(matches[3])
	if err != nil {
		return ChangeSummary{}, err
	}

	replace := replacementCount(output)

	return normalizeReplacementCounts(
		ChangeSummary{
			Add:     add,
			Change:  change,
			Destroy: destroy,
			Replace: replace,
		},
	), nil
}

func ParseApply(output string) (ChangeSummary, error) {
	output = stripANSI(output)

	matches := applySummaryPattern.FindStringSubmatch(
		output,
	)

	if len(matches) != 4 {
		return ChangeSummary{}, fmt.Errorf(
			"terraform apply summary not found",
		)
	}

	add, err := parseCount(matches[1])
	if err != nil {
		return ChangeSummary{}, err
	}

	change, err := parseCount(matches[2])
	if err != nil {
		return ChangeSummary{}, err
	}

	destroy, err := parseCount(matches[3])
	if err != nil {
		return ChangeSummary{}, err
	}

	replace := replacementCount(output)

	return normalizeReplacementCounts(
		ChangeSummary{
			Add:     add,
			Change:  change,
			Destroy: destroy,
			Replace: replace,
		},
	), nil
}

func ParseDestroy(output string) (ChangeSummary, error) {
	output = stripANSI(output)
	matches := destroySummaryPattern.FindStringSubmatch(
		output,
	)

	if len(matches) != 2 {
		return ChangeSummary{}, fmt.Errorf(
			"terraform destroy summary not found",
		)
	}

	destroy, err := parseCount(matches[1])
	if err != nil {
		return ChangeSummary{}, err
	}

	return ChangeSummary{
		Destroy: destroy,
	}, nil
}

func FormatSummary(summary ChangeSummary) string {
	return fmt.Sprintf(
		"+%d ~%d -%d -/+%d",
		summary.Add,
		summary.Change,
		summary.Destroy,
		summary.Replace,
	)
}

func replacementCount(output string) int {
	count := 0

	for _, line := range strings.Split(
		output,
		"\n",
	) {
		if strings.Contains(
			line,
			"must be replaced",
		) {
			count++
		}
	}

	return count
}

func normalizeReplacementCounts(
	summary ChangeSummary,
) ChangeSummary {
	if summary.Replace <= 0 {
		return summary
	}

	// Terraform's aggregate summary counts a replacement as both
	// an add and a destroy. WakeTrail presents replacements
	// separately, so remove that overlap from the plain counts.
	summary.Add -= summary.Replace
	summary.Destroy -= summary.Replace

	if summary.Add < 0 {
		summary.Add = 0
	}

	if summary.Destroy < 0 {
		summary.Destroy = 0
	}

	return summary
}

func stripANSI(output string) string {
	return ansiPattern.ReplaceAllString(
		output,
		"",
	)
}

func parseCount(value string) (int, error) {
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf(
			"invalid terraform resource count %q: %w",
			value,
			err,
		)
	}

	return count, nil
}

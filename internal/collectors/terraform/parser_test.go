package terraform

import "testing"

func TestParsePlan(t *testing.T) {
	output := `
Terraform will perform the following actions:

  # aws_instance.web will be created

  # aws_security_group.app will be updated in-place

Plan: 1 to add, 1 to change, 0 to destroy.
`

	got, err := ParsePlan(output)
	if err != nil {
		t.Fatal(err)
	}

	want := ChangeSummary{
		Add:     1,
		Change:  1,
		Destroy: 0,
		Replace: 0,
	}

	if got != want {
		t.Fatalf(
			"ParsePlan() = %+v, want %+v",
			got,
			want,
		)
	}
}

func TestParsePlanWithReplacement(t *testing.T) {
	output := `
Terraform will perform the following actions:

  # aws_instance.web will be created

  # aws_instance.worker must be replaced

  # aws_security_group.app will be updated in-place

  # aws_s3_bucket.old will be destroyed

Plan: 2 to add, 1 to change, 2 to destroy.
`

	got, err := ParsePlan(output)
	if err != nil {
		t.Fatal(err)
	}

	want := ChangeSummary{
		Add:     1,
		Change:  1,
		Destroy: 1,
		Replace: 1,
	}

	if got != want {
		t.Fatalf(
			"ParsePlan() = %+v, want %+v",
			got,
			want,
		)
	}

	formatted := FormatSummary(got)

	if formatted != "+1 ~1 -1 -/+1" {
		t.Fatalf(
			"FormatSummary() = %q, want %q",
			formatted,
			"+1 ~1 -1 -/+1",
		)
	}
}

func TestParseApply(t *testing.T) {
	output := `
  # aws_instance.worker must be replaced

Apply complete! Resources: 3 added, 2 changed, 2 destroyed.
`

	got, err := ParseApply(output)
	if err != nil {
		t.Fatal(err)
	}

	want := ChangeSummary{
		Add:     2,
		Change:  2,
		Destroy: 1,
		Replace: 1,
	}

	if got != want {
		t.Fatalf(
			"ParseApply() = %+v, want %+v",
			got,
			want,
		)
	}
}

func TestParseDestroy(t *testing.T) {
	output := `
Destroy complete! Resources: 4 destroyed.
`

	got, err := ParseDestroy(output)
	if err != nil {
		t.Fatal(err)
	}

	want := ChangeSummary{
		Destroy: 4,
	}

	if got != want {
		t.Fatalf(
			"ParseDestroy() = %+v, want %+v",
			got,
			want,
		)
	}
}

func TestParsePlanMissingSummary(t *testing.T) {
	_, err := ParsePlan(
		"Terraform has been successfully initialized!",
	)

	if err == nil {
		t.Fatal(
			"ParsePlan() expected error, got nil",
		)
	}
}

func TestParsePlanWithANSI(t *testing.T) {
	output := "\x1b[1mPlan:\x1b[0m \x1b[0m1 to add, 0 to change, 0 to destroy."

	got, err := ParsePlan(output)
	if err != nil {
		t.Fatal(err)
	}

	want := ChangeSummary{
		Add:     1,
		Change:  0,
		Destroy: 0,
		Replace: 0,
	}

	if got != want {
		t.Fatalf(
			"ParsePlan() = %+v, want %+v",
			got,
			want,
		)
	}
}

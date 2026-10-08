package docker

import "testing"

func TestParseDockerPS(t *testing.T) {
	input := []byte(
		"traefik\trunning\tUp 4 hours (healthy)\n" +
			"postgres\trunning\tUp 4 hours\n" +
			"api\trestarting\tRestarting (1) 5 seconds ago\n" +
			"worker\texited\tExited (1) 2 minutes ago\n",
	)

	containers, err := parseDockerPS(input)
	if err != nil {
		t.Fatalf("parseDockerPS() returned error: %v", err)
	}

	if len(containers) != 4 {
		t.Fatalf(
			"len(containers) = %d, want 4",
			len(containers),
		)
	}

	if containers[0].Name != "traefik" {
		t.Errorf(
			"containers[0].Name = %q, want %q",
			containers[0].Name,
			"traefik",
		)
	}

	if containers[0].State != "running" {
		t.Errorf(
			"containers[0].State = %q, want %q",
			containers[0].State,
			"running",
		)
	}

	if containers[0].Health != "healthy" {
		t.Errorf(
			"containers[0].Health = %q, want %q",
			containers[0].Health,
			"healthy",
		)
	}

	if containers[1].Health != "" {
		t.Errorf(
			"containers[1].Health = %q, want empty",
			containers[1].Health,
		)
	}

	if containers[2].State != "restarting" {
		t.Errorf(
			"containers[2].State = %q, want %q",
			containers[2].State,
			"restarting",
		)
	}

	if containers[3].State != "exited" {
		t.Errorf(
			"containers[3].State = %q, want %q",
			containers[3].State,
			"exited",
		)
	}
}

func TestHealthFromStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{
			name:   "healthy",
			status: "Up 20 minutes (healthy)",
			want:   "healthy",
		},
		{
			name:   "unhealthy",
			status: "Up 20 minutes (unhealthy)",
			want:   "unhealthy",
		},
		{
			name:   "starting",
			status: "Up 4 seconds (health: starting)",
			want:   "starting",
		},
		{
			name:   "no health check",
			status: "Up 20 minutes",
			want:   "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := healthFromStatus(test.status)

			if got != test.want {
				t.Errorf(
					"healthFromStatus(%q) = %q, want %q",
					test.status,
					got,
					test.want,
				)
			}
		})
	}
}

func TestParseDockerPSSkipsMalformedLines(t *testing.T) {
	input := []byte(
		"traefik\trunning\tUp 4 hours (healthy)\n" +
			"this line is malformed\n" +
			"postgres\trunning\tUp 4 hours\n",
	)

	containers, err := parseDockerPS(input)
	if err != nil {
		t.Fatalf("parseDockerPS() returned error: %v", err)
	}

	if len(containers) != 2 {
		t.Fatalf(
			"len(containers) = %d, want 2",
			len(containers),
		)
	}
}

func TestParseDockerPSComposeMetadata(t *testing.T) {
	input := []byte("web\trunning\tUp 2 seconds\tmonitoring\tfrontend\nplain\trunning\tUp 4 seconds\t\t\n")
	got, err := parseDockerPS(input)
	if err != nil || len(got) != 2 || got[0].ComposeProject != "monitoring" || got[0].ComposeService != "frontend" || got[1].ComposeProject != "" {
		t.Fatalf("metadata parsing: %+v, %v", got, err)
	}
}

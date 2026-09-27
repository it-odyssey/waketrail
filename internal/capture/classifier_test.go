package capture

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    Mode
	}{
		{
			name:    "pwd",
			command: "pwd",
			want:    ModeOutput,
		},
		{
			name:    "curl",
			command: "curl --fail http://localhost:8080/health",
			want:    ModeOutput,
		},
		{
			name:    "grep",
			command: `grep "ERROR" app.log`,
			want:    ModeOutput,
		},
		{
			name:    "ripgrep",
			command: `rg "failed" .`,
			want:    ModeOutput,
		},
		{
			name:    "git status",
			command: "git status --short --branch",
			want:    ModeOutput,
		},
		{
			name:    "git diff",
			command: "git diff",
			want:    ModeOutput,
		},
		{
			name:    "git commit",
			command: `git commit -m "test"`,
			want:    ModeNone,
		},
		{
			name:    "docker ps",
			command: "docker ps -a",
			want:    ModeOutput,
		},
		{
			name:    "docker inspect",
			command: "docker inspect nginx",
			want:    ModeOutput,
		},
		{
			name:    "docker logs",
			command: "docker logs nginx",
			want:    ModeBounded,
		},
		{
			name:    "docker stop",
			command: "docker stop nginx",
			want:    ModeNone,
		},
		{
			name:    "docker compose ps",
			command: "docker compose ps",
			want:    ModeOutput,
		},
		{
			name:    "docker compose logs",
			command: "docker compose logs api",
			want:    ModeBounded,
		},
		{
			name:    "systemctl status",
			command: "systemctl status nginx",
			want:    ModeOutput,
		},
		{
			name:    "systemctl restart",
			command: "systemctl restart nginx",
			want:    ModeNone,
		},
		{
			name:    "journalctl",
			command: "journalctl -u nginx",
			want:    ModeBounded,
		},
		{
			name:    "kubectl get",
			command: "kubectl get pods",
			want:    ModeOutput,
		},
		{
			name:    "kubectl describe",
			command: "kubectl describe pod api",
			want:    ModeOutput,
		},
		{
			name:    "kubectl logs",
			command: "kubectl logs api",
			want:    ModeBounded,
		},
		{
			name:    "terraform plan",
			command: "terraform plan",
			want:    ModeBounded,
		},
		{
			name:    "terraform output",
			command: "terraform output",
			want:    ModeOutput,
		},
		{
			name:    "tofu plan",
			command: "tofu plan",
			want:    ModeBounded,
		},
		{
			name:    "ip addr",
			command: "ip addr show",
			want:    ModeOutput,
		},
		{
			name:    "ping",
			command: "ping -c 4 1.1.1.1",
			want:    ModeBounded,
		},
		{
			name:    "sudo docker ps",
			command: "sudo docker ps",
			want:    ModeOutput,
		},
		{
			name:    "absolute curl path",
			command: "/usr/bin/curl http://localhost",
			want:    ModeOutput,
		},
		{
			name:    "cd",
			command: "cd /tmp",
			want:    ModeNone,
		},
		{
			name:    "export",
			command: "export FOO=bar",
			want:    ModeNone,
		},
		{
			name:    "variable assignment",
			command: "FOO=bar",
			want:    ModeNone,
		},
		{
			name:    "vim",
			command: "vim nginx.conf",
			want:    ModeNone,
		},
		{
			name:    "ssh",
			command: "ssh server",
			want:    ModeNone,
		},
		{
			name:    "unknown command",
			command: "some-company-tool deploy",
			want:    ModeNone,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Classify(test.command)

			if got != test.want {
				t.Errorf(
					"Classify(%q) = %q, want %q",
					test.command,
					got,
					test.want,
				)
			}
		})
	}
}

func TestClassifyWgetOutputFile(t *testing.T) {
	tests := []string{
		"wget -O page.html https://example.com",
		"wget --output-document page.html https://example.com",
		"wget --output-document=page.html https://example.com",
	}

	for _, command := range tests {
		t.Run(command, func(t *testing.T) {
			got := Classify(command)

			if got != ModeNone {
				t.Errorf(
					"Classify(%q) = %q, want %q",
					command,
					got,
					ModeNone,
				)
			}
		})
	}
}

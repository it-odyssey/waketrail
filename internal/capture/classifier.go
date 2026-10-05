package capture

import (
	"path/filepath"
	"strings"
)

type Mode string

const (
	ModeNone    Mode = "none"
	ModeOutput  Mode = "output"
	ModeBounded Mode = "bounded"
)

func Classify(command string) Mode {
	fields := strings.Fields(command)

	if len(fields) == 0 {
		return ModeNone
	}

	fields = stripCommandPrefixes(fields)

	if len(fields) == 0 {
		return ModeNone
	}

	name := filepath.Base(fields[0])

	switch name {
	case "pwd",
		"grep",
		"rg",
		"df",
		"free",
		"ss",
		"dig",
		"nslookup",
		"whoami",
		"hostname",
		"uname":
		return ModeOutput

	case "curl":
		return ModeOutput

	case "wget":
		return classifyWget(fields)

	case "git":
		return classifyGit(fields)

	case "docker":
		return classifyDocker(fields)

	case "systemctl":
		return classifySystemctl(fields)

	case "journalctl":
		return ModeBounded

	case "kubectl":
		return classifyKubectl(fields)

	case "terraform", "tofu":
		return classifyTerraform(fields)

	case "ip":
		return classifyIP(fields)

	case "ping":
		return ModeBounded

	default:
		return ModeNone
	}
}

func stripCommandPrefixes(fields []string) []string {
	for len(fields) > 0 {
		switch fields[0] {
		case "sudo", "command":
			fields = fields[1:]

			for len(fields) > 0 &&
				strings.HasPrefix(fields[0], "-") {
				fields = fields[1:]
			}

		default:
			return fields
		}
	}

	return fields
}

func classifyGit(fields []string) Mode {
	subcommand := subcommand(fields)

	switch subcommand {
	case "status",
		"diff",
		"log",
		"show",
		"branch",
		"rev-parse":
		return ModeOutput

	default:
		return ModeNone
	}
}

func classifyDocker(fields []string) Mode {
	subcommand := subcommand(fields)

	switch subcommand {
	case "ps",
		"inspect",
		"stats",
		"images",
		"info":
		return ModeOutput

	case "logs":
		return ModeBounded

	case "compose":
		return classifyDockerCompose(fields)

	default:
		return ModeNone
	}
}

func classifyDockerCompose(fields []string) Mode {
	if len(fields) < 3 {
		return ModeNone
	}

	switch fields[2] {
	case "ps",
		"config":
		return ModeOutput

	case "logs":
		return ModeBounded

	default:
		return ModeNone
	}
}

func classifySystemctl(fields []string) Mode {
	subcommand := subcommand(fields)

	switch subcommand {
	case "status",
		"is-active",
		"is-failed",
		"show",
		"list-units":
		return ModeOutput

	default:
		return ModeNone
	}
}

func classifyKubectl(fields []string) Mode {
	subcommand := subcommand(fields)

	switch subcommand {
	case "get",
		"describe":
		return ModeOutput

	case "logs",
		"events":
		return ModeBounded

	default:
		return ModeNone
	}
}

func classifyTerraform(fields []string) Mode {
	subcommand := subcommand(fields)

	switch subcommand {
	case "show",
		"output",
		"validate",
		"version":
		return ModeOutput

	case "plan",
		"apply",
		"destroy":
		return ModeBounded

	case "state":
		return classifyTerraformState(fields)

	default:
		return ModeNone
	}
}

func classifyTerraformState(fields []string) Mode {
	if len(fields) < 3 {
		return ModeNone
	}

	switch fields[2] {
	case "list",
		"show":
		return ModeOutput

	default:
		return ModeNone
	}
}

func classifyIP(fields []string) Mode {
	subcommand := subcommand(fields)

	switch subcommand {
	case "addr",
		"address",
		"route",
		"link":
		return ModeOutput

	default:
		return ModeNone
	}
}

func classifyWget(fields []string) Mode {
	for _, field := range fields[1:] {
		if field == "-O" ||
			field == "--output-document" ||
			strings.HasPrefix(
				field,
				"--output-document=",
			) {
			return ModeNone
		}
	}

	return ModeOutput
}

func subcommand(fields []string) string {
	if len(fields) < 2 {
		return ""
	}

	for _, field := range fields[1:] {
		if strings.HasPrefix(field, "-") {
			continue
		}

		return field
	}

	return ""
}

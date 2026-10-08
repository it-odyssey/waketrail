package capture

import (
	"path/filepath"
	"strings"
)

// OutputAllowed is a conservative guard, not a shell parser. Complex syntax is
// metadata-only so a safe first command cannot authorize secret output later in
// the same pipeline, substitution, or command list.
func OutputAllowed(command string) bool {
	if strings.ContainsAny(command, "\n\r;&|<>`") || strings.Contains(command, "$(") {
		return false
	}
	fields := stripCommandPrefixes(strings.Fields(command))
	if len(fields) == 0 {
		return false
	}
	// Kubernetes object specs/descriptions can expose literal environment values.
	// Retain only ordinary inventory tables from known resource types.
	if filepath.Base(fields[0]) == "kubectl" {
		return kubectlCaptureVerb(fields[1:]) != ""
	}
	return true
}

func kubectlCaptureVerb(fields []string) string {
	verb := ""
	resource := ""
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--context", "--kubeconfig", "--cluster", "--user", "--server", "--token", "--certificate-authority", "--client-certificate", "--client-key", "--request-timeout", "--namespace", "-n", "--selector", "-l", "--field-selector", "--container", "-c", "--since", "--since-time", "--tail", "--limit-bytes", "--sort-by":
			if i+1 >= len(fields) {
				return ""
			}
			i++
			continue
		case "-o", "--output":
			if i+1 >= len(fields) || fields[i+1] != "wide" {
				return ""
			}
			i++
			continue
		}
		if strings.HasPrefix(field, "--output=") {
			if field != "--output=wide" {
				return ""
			}
			continue
		}
		if strings.HasPrefix(field, "-o") && field != "-o" {
			if field != "-owide" && field != "-o=wide" {
				return ""
			}
			continue
		}
		if strings.HasPrefix(field, "--") && strings.Contains(field, "=") {
			name, _, _ := strings.Cut(field, "=")
			switch name {
			case "--context", "--kubeconfig", "--cluster", "--user", "--server", "--token", "--certificate-authority", "--client-certificate", "--client-key", "--request-timeout", "--namespace", "--selector", "--field-selector", "--container", "--since", "--since-time", "--tail", "--limit-bytes", "--sort-by":
				continue
			default:
				return ""
			}
		}
		if strings.HasPrefix(field, "-") {
			switch field {
			case "--all-namespaces", "-A", "--no-headers", "--show-labels", "--ignore-not-found", "--watch", "-w", "--watch-only", "--follow", "-f", "--previous", "-p", "--timestamps":
				continue
			default:
				return ""
			}
		}
		if verb == "" {
			verb = field
			continue
		}
		if resource == "" {
			resource = field
			continue
		}
		// Reject extra positional resource selectors conservatively.
		if strings.ContainsAny(field, ",/") {
			return ""
		}
	}
	if verb == "logs" || verb == "events" {
		return verb
	}
	if verb != "get" {
		return ""
	}
	for _, part := range strings.Split(resource, ",") {
		kind := strings.SplitN(part, "/", 2)[0]
		switch kind {
		case "pod", "pods", "po", "deployment", "deployments", "deploy", "statefulset", "statefulsets", "sts", "daemonset", "daemonsets", "ds", "node", "nodes", "no", "service", "services", "svc", "namespace", "namespaces", "ns", "endpoints", "ep", "ingress", "ingresses", "ing", "replicaset", "replicasets", "rs":
		default:
			return ""
		}
	}
	if resource == "" {
		return ""
	}
	return verb
}

// RecordingMode enforces classification again at persistence time, even if an
// older hook or direct recorder invocation supplies a more permissive mode.
func RecordingMode(command, requested string) Mode {
	allowed := Classify(command)
	if requested != string(ModeOutput) && requested != string(ModeBounded) {
		return ModeNone
	}
	if allowed == ModeNone {
		return ModeNone
	}
	if requested == string(ModeBounded) || allowed == ModeBounded {
		return ModeBounded
	}
	return ModeOutput
}

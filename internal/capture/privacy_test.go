package capture

import "testing"

func TestSecretProneAndComplexCommandsAreMetadataOnly(t *testing.T) {
	for _, command := range []string{
		"kubectl get secret", "kubectl get secrets -o yaml", "kubectl -n default get secret/db -o json",
		"kubectl get pods,secrets", "kubectl get configmap", "kubectl describe pod web",
		"kubectl get pods -o yaml", "kubectl get pods --output=json", "kubectl get pods -ojson",
		"kubectl get pods -o custom-columns=PASSWORD:.spec.containers[*].env[*].value",
		"kubectl get --raw=/api/v1/secrets", "kubectl --raw=/api/v1/secrets get pods",
		"docker inspect web", "docker container inspect web", "docker compose -p demo config",
		"docker compose --file=compose.yaml config", "terraform output", "terraform show -json",
		"terraform state show aws_instance.web", "tofu -chdir=infra output -json",
		"git log -p", "git log -1p", "git log --patch-with-stat", "git log --cc", "git log --patch", "git diff", "git show HEAD", "git -C repo diff", "sudo docker inspect web",
		"curl http://localhost:8080/health", "wget -O - https://example.test",
		"docker ps; kubectl get secrets", "docker ps | cat secret.txt", "docker ps && cat secret.txt",
		"docker ps $(cat secret.txt)", "docker ps `cat secret.txt`", "docker ps\ncat secret.txt",
		"cat <<< unlabelled-credential", "docker ps > output.txt",
	} {
		t.Run(command, func(t *testing.T) {
			if got := Classify(command); got != ModeNone {
				t.Fatalf("Classify() = %q", got)
			}
			if got := RecordingMode(command, "output"); got != ModeNone {
				t.Fatalf("recorder bypass = %q", got)
			}
		})
	}
}

func TestPrivacyPolicyRetainsDiagnosticCapture(t *testing.T) {
	for _, command := range []string{
		"docker ps -a", "docker compose -p demo ps -a", "git status --short",
		"kubectl get pods", "kubectl --context lab -n default get pods -o wide",
		"kubectl get pods,deployments -A", "kubectl get deployment/web --namespace=default",
		"kubectl get pods --field-selector=status.phase=Running", "terraform state list",
	} {
		if got := Classify(command); got != ModeOutput {
			t.Errorf("%q = %q", command, got)
		}
	}
	for _, command := range []string{"terraform apply -auto-approve", "docker logs web", "kubectl -n default logs web --tail=20"} {
		if got := RecordingMode(command, "output"); got != ModeBounded {
			t.Errorf("%q = %q", command, got)
		}
	}
	if got := RecordingMode("docker ps", "none"); got != ModeNone {
		t.Fatal("requested metadata-only recording ignored")
	}
}

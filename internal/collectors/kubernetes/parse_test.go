package kubernetes

import (
	"encoding/json"
	"testing"
)

func TestParseSnapshotMixedResources(t *testing.T) {
	input := []byte(`{
		"apiVersion": "v1",
		"items": [
			{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "api-123",
					"namespace": "default"
				},
				"status": {
					"phase": "Running",
					"containerStatuses": [
						{
							"ready": true,
							"state": {
								"running": {}
							}
						}
					]
				}
			},
			{
				"apiVersion": "apps/v1",
				"kind": "Deployment",
				"metadata": {
					"name": "api",
					"namespace": "default"
				},
				"spec": {
					"replicas": 3
				},
				"status": {
					"updatedReplicas": 3,
					"readyReplicas": 3,
					"availableReplicas": 3
				}
			},
			{
				"apiVersion": "apps/v1",
				"kind": "StatefulSet",
				"metadata": {
					"name": "postgres",
					"namespace": "data"
				},
				"spec": {
					"replicas": 2
				},
				"status": {
					"currentReplicas": 2,
					"updatedReplicas": 2,
					"readyReplicas": 2
				}
			},
			{
				"apiVersion": "apps/v1",
				"kind": "DaemonSet",
				"metadata": {
					"name": "agent",
					"namespace": "monitoring"
				},
				"status": {
					"desiredNumberScheduled": 3,
					"currentNumberScheduled": 3,
					"updatedNumberScheduled": 3,
					"numberReady": 3,
					"numberAvailable": 3,
					"numberMisscheduled": 0
				}
			},
			{
				"apiVersion": "v1",
				"kind": "Node",
				"metadata": {
					"name": "worker-1"
				},
				"spec": {
					"unschedulable": false
				},
				"status": {
					"conditions": [
						{
							"type": "Ready",
							"status": "True"
						},
						{
							"type": "MemoryPressure",
							"status": "False"
						},
						{
							"type": "DiskPressure",
							"status": "False"
						},
						{
							"type": "PIDPressure",
							"status": "False"
						}
					]
				}
			}
		]
	}`)

	snapshot, err := parseSnapshot(input)
	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot.Pods) != 1 {
		t.Fatalf(
			"pods = %d, want 1",
			len(snapshot.Pods),
		)
	}

	if len(snapshot.Deployments) != 1 {
		t.Fatalf(
			"deployments = %d, want 1",
			len(snapshot.Deployments),
		)
	}

	if len(snapshot.StatefulSets) != 1 {
		t.Fatalf(
			"statefulsets = %d, want 1",
			len(snapshot.StatefulSets),
		)
	}

	if len(snapshot.DaemonSets) != 1 {
		t.Fatalf(
			"daemonsets = %d, want 1",
			len(snapshot.DaemonSets),
		)
	}

	if len(snapshot.Nodes) != 1 {
		t.Fatalf(
			"nodes = %d, want 1",
			len(snapshot.Nodes),
		)
	}

	pod := snapshot.Pods[0]

	if pod.Namespace != "default" ||
		pod.Name != "api-123" ||
		pod.Phase != "Running" ||
		pod.Ready != 1 ||
		pod.Total != 1 {
		t.Fatalf(
			"unexpected pod state: %+v",
			pod,
		)
	}

	deployment := snapshot.Deployments[0]

	if deployment.Desired != 3 ||
		deployment.Updated != 3 ||
		deployment.Ready != 3 ||
		deployment.Available != 3 {
		t.Fatalf(
			"unexpected deployment state: %+v",
			deployment,
		)
	}

	statefulSet := snapshot.StatefulSets[0]

	if statefulSet.Desired != 2 ||
		statefulSet.Current != 2 ||
		statefulSet.Updated != 2 ||
		statefulSet.Ready != 2 {
		t.Fatalf(
			"unexpected statefulset state: %+v",
			statefulSet,
		)
	}

	daemonSet := snapshot.DaemonSets[0]

	if daemonSet.Desired != 3 ||
		daemonSet.Current != 3 ||
		daemonSet.Updated != 3 ||
		daemonSet.Ready != 3 ||
		daemonSet.Available != 3 ||
		daemonSet.Misscheduled != 0 {
		t.Fatalf(
			"unexpected daemonset state: %+v",
			daemonSet,
		)
	}

	node := snapshot.Nodes[0]

	if node.Name != "worker-1" ||
		node.Ready != "True" ||
		node.Unschedulable ||
		node.MemoryPressure ||
		node.DiskPressure ||
		node.PIDPressure {
		t.Fatalf(
			"unexpected node state: %+v",
			node,
		)
	}
}

func TestParsePodCrashLoopBackOff(t *testing.T) {
	raw := json.RawMessage(`{
		"metadata": {
			"name": "api-123",
			"namespace": "default"
		},
		"status": {
			"phase": "Running",
			"containerStatuses": [
				{
					"ready": false,
					"state": {
						"waiting": {
							"reason": "CrashLoopBackOff"
						}
					}
				}
			]
		}
	}`)

	state, err := parsePod(raw)
	if err != nil {
		t.Fatal(err)
	}

	if state.Reason != "CrashLoopBackOff" {
		t.Fatalf(
			"reason = %q, want CrashLoopBackOff",
			state.Reason,
		)
	}
}

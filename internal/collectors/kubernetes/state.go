package kubernetes

type Snapshot struct {
	Pods         []PodState
	Deployments  []DeploymentState
	StatefulSets []StatefulSetState
	DaemonSets   []DaemonSetState
	Nodes        []NodeState
}

type PodState struct {
	Namespace string
	Name      string
	Phase     string
	Ready     int
	Total     int
	Reason    string
}

type DeploymentState struct {
	Namespace string
	Name      string
	Desired   int32
	Updated   int32
	Ready     int32
	Available int32
}

type StatefulSetState struct {
	Namespace string
	Name      string
	Desired   int32
	Current   int32
	Updated   int32
	Ready     int32
}

type DaemonSetState struct {
	Namespace    string
	Name         string
	Desired      int32
	Current      int32
	Updated      int32
	Ready        int32
	Available    int32
	Misscheduled int32
}

type NodeState struct {
	Name           string
	Ready          string
	Unschedulable  bool
	MemoryPressure bool
	DiskPressure   bool
	PIDPressure    bool
}

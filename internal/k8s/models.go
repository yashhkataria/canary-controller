package k8s

import "k8s.io/client-go/kubernetes"

type Client struct {
	cs kubernetes.Interface
}

type DeploymentStatus struct {
	Name               string
	Container          string
	Image              string
	RolloutLabel       string
	Desired            int32
	Replicas           int32
	Ready              int32
	Updated            int32
	Available          int32
	Generation         int64
	ObservedGeneration int64
}

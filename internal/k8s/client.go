package k8s

import (
	"errors"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func New(kubeconfig string) (*Client, error) {
	cfg, err := restConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	cfg.Timeout = requestTimeout
	cfg.UserAgent = userAgent

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	return &Client{cs: cs}, nil
}

func NewFromClientset(cs kubernetes.Interface) *Client {
	return &Client{cs: cs}
}

func restConfig(kubeconfig string) (*rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, rest.ErrNotInCluster) {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		rules.ExplicitPath = kubeconfig
	}
	cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("not running in a cluster and no usable kubeconfig: %w", err)
	}
	return cfg, nil
}

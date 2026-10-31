package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (c *Client) Status(ctx context.Context, namespace, name, container string) (DeploymentStatus, error) {
	d, err := c.get(ctx, namespace, name)
	if err != nil {
		return DeploymentStatus{}, err
	}
	ctr, err := pickContainer(d, container)
	if err != nil {
		return DeploymentStatus{}, err
	}
	desired := int32(1)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	return DeploymentStatus{
		Name:               d.Name,
		Container:          ctr.Name,
		Image:              ctr.Image,
		RolloutLabel:       d.Spec.Template.Labels[RolloutLabel],
		Desired:            desired,
		Replicas:           d.Status.Replicas,
		Ready:              d.Status.ReadyReplicas,
		Updated:            d.Status.UpdatedReplicas,
		Available:          d.Status.AvailableReplicas,
		Generation:         d.Generation,
		ObservedGeneration: d.Status.ObservedGeneration,
	}, nil
}

func (c *Client) SetReplicas(ctx context.Context, namespace, name string, replicas int32) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	patch, err := json.Marshal(map[string]any{"spec": map[string]any{"replicas": replicas}})
	if err != nil {
		return err
	}
	_, err = c.cs.AppsV1().Deployments(namespace).Patch(ctx, name, types.MergePatchType, patch,
		metav1.PatchOptions{FieldManager: fieldManager}, subresourceScale)
	if err != nil {
		return mapError(err, "scale "+namespace+"/"+name)
	}
	return nil
}

func (c *Client) SetImage(ctx context.Context, namespace, name, container, image string, labels map[string]string) error {
	d, err := c.get(ctx, namespace, name)
	if err != nil {
		return err
	}
	ctr, err := pickContainer(d, container)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	template := map[string]any{"spec": map[string]any{
		"containers": []map[string]string{{"name": ctr.Name, "image": image}},
	}}
	if len(labels) > 0 {
		template["metadata"] = map[string]any{"labels": labels}
	}
	patch, err := json.Marshal(map[string]any{"spec": map[string]any{"template": template}})
	if err != nil {
		return err
	}
	_, err = c.cs.AppsV1().Deployments(namespace).Patch(ctx, name, types.StrategicMergePatchType, patch,
		metav1.PatchOptions{FieldManager: fieldManager})
	if err != nil {
		return mapError(err, "set image on "+namespace+"/"+name)
	}
	return nil
}

func (c *Client) get(ctx context.Context, namespace, name string) (*appsv1.Deployment, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	d, err := c.cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, mapError(err, "get "+namespace+"/"+name)
	}
	return d, nil
}

func pickContainer(d *appsv1.Deployment, want string) (*corev1.Container, error) {
	cs := d.Spec.Template.Spec.Containers
	for i := range cs {
		if cs[i].Name == want {
			return &cs[i], nil
		}
	}
	if len(cs) == 1 {
		return &cs[0], nil
	}
	return nil, fmt.Errorf("deployment %s has %d containers and none is named %q", d.Name, len(cs), want)
}

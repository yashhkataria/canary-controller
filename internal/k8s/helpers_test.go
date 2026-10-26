package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func deployment(name string, replicas int32, containers ...corev1.Container) *appsv1.Deployment {
	if len(containers) == 0 {
		containers = []corev1.Container{{Name: "demo", Image: "demo:v1"}}
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Generation: 3, Labels: map[string]string{"app": "demo"}},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: containers}},
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 3,
			Replicas:           replicas,
			ReadyReplicas:      replicas,
			UpdatedReplicas:    replicas,
			AvailableReplicas:  replicas,
		},
	}
}

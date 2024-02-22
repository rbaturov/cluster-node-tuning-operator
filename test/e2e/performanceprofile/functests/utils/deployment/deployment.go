package deployment

import (
	"fmt"
	"context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	
)

func NewTestDeployment(replicas int32, podLabels map[string]string, nodeSelector map[string]string, namespace, name, image string, command, args []string) *appsv1.Deployment {
	var zero int64
	podSpec := corev1.PodSpec{
		TerminationGracePeriodSeconds: &zero,
		Containers: []corev1.Container{
			{
				Name:    name + "-cnt",
				Image:   image,
				Command: command,
			},
		},
		RestartPolicy: corev1.RestartPolicyAlways,
	}
	dp := NewTestDeploymentWithPodSpec(replicas, podLabels, nodeSelector, namespace, name, podSpec)
	if nodeSelector != nil {
		dp.Spec.Template.Spec.NodeSelector = nodeSelector
	}
	return dp
}

func NewTestDeploymentWithPodSpec(replicas int32, podLabels map[string]string, nodeSelector map[string]string, namespace, name string, podSpec corev1.PodSpec) *appsv1.Deployment {
	dp := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: podLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: podSpec,
			},
		},
	}
	if nodeSelector != nil {
		dp.Spec.Template.Spec.NodeSelector = nodeSelector
	}
	return dp
}

func IsDeploymentReady(ctx context.Context, client client.Client, listOptions *client.ListOptions, podList *corev1.PodList, dp *appsv1.Deployment) (bool, error) {
	if err := client.List(ctx, podList, listOptions); err != nil || len(podList.Items) == 0{
		return false, err
	}
	fmt.Printf("Printing number of pods:%d", len(podList.Items))

	for _, pod := range podList.Items {
		if !isPodReady(&pod) {
			return false, nil
		}
	}

	return true, nil
}

func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
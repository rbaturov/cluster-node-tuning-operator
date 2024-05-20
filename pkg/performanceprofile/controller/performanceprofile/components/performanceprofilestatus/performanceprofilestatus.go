package runtimeclass

import (
	performancev2 "github.com/openshift/cluster-node-tuning-operator/pkg/apis/performanceprofile/v2"
	"github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/components"
)

// New returns a new performane profile status configmap object
func New(status *performancev2.PerformanceProfileStatus) *corev1.ConfigMap {
	name := components.GetComponentName(profile.Name, components.ComponentNamePrefix)
	return &performancev2.PerformanceProfileStatus{}
}



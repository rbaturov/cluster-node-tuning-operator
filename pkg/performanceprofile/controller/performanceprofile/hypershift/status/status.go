package status

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	performancev2 "github.com/openshift/cluster-node-tuning-operator/pkg/apis/performanceprofile/v2"
	tunedv1 "github.com/openshift/cluster-node-tuning-operator/pkg/apis/tuned/v1"
	"github.com/openshift/cluster-node-tuning-operator/pkg/operator"
	"github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/hypershift"
	handler "github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/hypershift/components"
	"github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/status"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
)

const (
	PPstatusConfigMapPrefix = "status"
)

func createOrUpdateStatusConfigMap(ctx context.Context, cli client.Client, cm *corev1.ConfigMap, cond *conditionsv1.Condition) error {
	err := cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)

	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to read ConfigMap %q: %w", cm.Name, err)
	}

	data := map[string]string{
		"type":    string(cond.Type),
		"status":  string(cond.Status),
		"message": cond.Message,
		"reason":  cond.Reason,
	}

	if k8serrors.IsNotFound(err) {
		cm.Data = data
		return cli.Create(ctx, cm)
	}

	// Check if update is needed
	for key, value := range data {
		if cm.Data[key] != value {
			cm.Data = data
			return cli.Update(ctx, cm)
		}
	}

	return nil
}

func ppStatusConfigMap(performanceProfileCM *corev1.ConfigMap, profileName string, scheme *runtime.Scheme) (*corev1.ConfigMap, error) {
	nodePoolNamespacedName, ok := performanceProfileCM.Annotations[operator.HypershiftNodePoolLabel]
	if !ok {
		return nil, fmt.Errorf("annotation %q not found in ConfigMap %q annotations", operator.HypershiftNodePoolLabel, client.ObjectKeyFromObject(performanceProfileCM).String())
	}
	name := fmt.Sprintf("%s-%s", PPstatusConfigMapPrefix, performanceProfileCM.Name)
	cm := handler.ConfigMapMeta(name, profileName, performanceProfileCM.GetNamespace(), nodePoolNamespacedName)
	err := controllerutil.SetControllerReference(performanceProfileCM, cm, scheme)
	if err != nil {
		return nil, err
	}
	cm.Labels[operator.NtoGeneratedPerformanceProfileStatusLabel] = "true"

	return cm, nil
}

func extractAndDecodeProfile(performanceProfileCM *corev1.ConfigMap, object client.Object, scheme *runtime.Scheme) (*performancev2.PerformanceProfile, error) {
	s, ok := performanceProfileCM.Data[hypershift.TuningKey]
	if !ok {
		return nil, fmt.Errorf("key named %q not found in ConfigMap %q", hypershift.TuningKey, client.ObjectKeyFromObject(object).String())
	}

	profile := &performancev2.PerformanceProfile{}
	if err := hypershift.DecodeManifest([]byte(s), scheme, profile); err != nil {
		return nil, err
	}

	return profile, nil
}

func getTunedConditions(ctx context.Context, cli client.Client) ([]conditionsv1.Condition, error) {
	tunedProfileList := &tunedv1.ProfileList{}
	if err := cli.List(ctx, tunedProfileList); err != nil {
		klog.Errorf("Cannot list Tuned Profiles: %v", err)
		return nil, err
	}
	if len(tunedProfileList.Items) == 0 {
		return nil, fmt.Errorf("no tuned profiles has been found")
	}
	messageString := status.GetTunedProfilesMessage(tunedProfileList.Items)
	if len(messageString) == 0 {
		return nil, nil
	}

	return status.GetDegradedConditions(status.ConditionReasonTunedDegraded, messageString), nil
}

// Given that the performance profile status always has one of the conditions Available, Progressing, or Degraded set to true,
// we can extract this condition and compose a single condition that will represent the status and will reflects the overall performance profile application status.
// This will be used as an input for the nodepool.conditions.PerformanceProfileAppliedSuccessfully on hypershift.
func extractDominantCondition(conditions []conditionsv1.Condition) (*conditionsv1.Condition, error) {
	for _, condition := range conditions {
		if condition.Type != conditionsv1.ConditionUpgradeable && condition.Status == corev1.ConditionTrue {
			return &condition, nil
		}
	}
	// This should never happen. Exactly one of the conditions Available, Progressing or Degraded should be true.
	return  nil, fmt.Errorf("no Available, Progressing or Degraded condition has been found")
}

func StatusToYAML(status *performancev2.PerformanceProfileStatus) ([]byte, error) {
	jsonData, err := json.Marshal(status)
	if err != nil {
		return nil, err
	}
	yamlData, err := yaml.JSONToYAML(jsonData)
	if err != nil {
		return nil, err
	}
	return yamlData, nil
}

func StatusFromYAML(yamlData []byte, status *performancev2.PerformanceProfileStatus) error {
	jsonData, err := yaml.YAMLToJSON(yamlData)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonData, status)
	if err != nil {
		return err
	}
	return nil
}

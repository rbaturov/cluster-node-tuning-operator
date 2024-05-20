package status

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	performancev2 "github.com/openshift/cluster-node-tuning-operator/pkg/apis/performanceprofile/v2"
	"github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/components"
	"github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/hypershift"
	handler "github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/hypershift/components"
	"github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/status"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ status.Writer = &writer{}

type writer struct {
	controlPlaneClient client.Client
	dataPlaneClient    client.Client
	scheme             *runtime.Scheme
}

func NewWriter(controlPlaneClient client.Client, dataPlaneClient client.Client, scheme *runtime.Scheme) status.Writer {
	return &writer{controlPlaneClient: controlPlaneClient, dataPlaneClient: dataPlaneClient, scheme: scheme}
}

func (w *writer) Update(ctx context.Context, object client.Object, conditions []conditionsv1.Condition) error {

	// need to a performanceprofilev2.PerformanceProfileStatus

	perfomanceProfileCM, ok := object.(*corev1.ConfigMap)
	if !ok {
		return fmt.Errorf("wrong type conversion; want=ConfigMap got=%T", object)
	}
	// Convert to PP, pass only status

	s, ok := perfomanceProfileCM.Data[hypershift.TuningKey]
	if !ok {
		return fmt.Errorf("key named %q not found in ConfigMap %q", hypershift.TuningKey, client.ObjectKeyFromObject(object).String())
	}

	profile := &performancev2.PerformanceProfile{}
	if err := hypershift.DecodeManifest([]byte(s), w.scheme, profile); err != nil {
		return err
	}

	// // Get the PP status configMap by the pp name
	PPstatusCM := &corev1.ConfigMap{}
	err := cli.Get(ctx, client.ObjectKeyFromObject(cm), PPstatusCM)
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to read configmap %q: %w", cm.Name, err)
	} else if k8serrors.IsNotFound(err) {
		//create
		if err := cli.Create(ctx, cm); err != nil {
			return fmt.Errorf("failed to create configmap %q: %w", cm.Name, err)
		}
	} else {
		// update
		updateFunc(cm, tcm)
		if err := cli.Update(ctx, tcm); err != nil {
			return fmt.Errorf("failed to update configmap %q: %w", cm.Name, err)
		}
	}
	return 
	// if err != nil && !k8serrors.IsNotFound(err) {
	// 	return fmt.Errorf("failed to read configmap %q: %w", cm.Name, err)
	// } else if k8serrors.IsNotFound(err) {
	return w.update(ctx, PP_status, profile.Name, conditions)

}

func (w *writer) updateDegradedCondition(instance client.Object, conditionState string, conditionError error) error {
	conditions := status.GetDegradedConditions(conditionState, conditionError.Error())
	if err := w.Update(context.TODO(), instance, conditions); err != nil {
		klog.Errorf("failed to update performance profile %q status: %v", instance.GetName(), err)
		return err
	}
	return conditionError
}

func (w *writer) UpdateOwnedConditions(ctx context.Context, object client.Object) error {
	performanceProfileCM, ok := object.(*corev1.ConfigMap)
	if !ok {
		return fmt.Errorf("wrong type conversion; want=ConfigMap got=%T", object)
	}

	s, ok := performanceProfileCM.Data[hypershift.TuningKey]
	if !ok {
		return fmt.Errorf("key named %q not found in ConfigMap %q", hypershift.TuningKey, client.ObjectKeyFromObject(object).String())
	}

	profile := &performancev2.PerformanceProfile{}
	if err := hypershift.DecodeManifest([]byte(s), w.scheme, profile); err != nil {
		return err
	}

	// // get kubelet false condition
	// // TODO: make sure the control plane client needed here
	// conditions, err := status.GetKubeletConditionsByProfile(ctx, w.dataPlaneClient, profile.Name)
	// if err != nil {
	// 	return w.updateDegradedCondition(profile, status.ConditionFailedGettingKubeletStatus, err)
	// }

	// Check if this works ?

	conditions, err := status.GetTunedConditionsByProfile(ctx, w.dataPlaneClient, profile)
	if err != nil {
		return w.updateDegradedCondition(profile, status.ConditionFailedGettingTunedProfileStatus, err)
	}

	// if conditions were not added then set as available
	if conditions == nil {
		conditions = status.GetAvailableConditions("")
	}
	return w.Update(ctx, object, conditions)
}

func (w *writer) update(ctx context.Context, performanceProfileStatusCM *corev1.ConfigMap, profileName string, conditions []conditionsv1.Condition) error {
	statusRaw, ok := performanceProfileStatusCM.Data["status"]
	if !ok {
		return fmt.Errorf("status not found in PerformanceProfileStatus ConfigMap")
	}
	status := &performancev2.PerformanceProfileStatus{}
	if err := json.Unmarshal([]byte(statusRaw), status); err != nil {
		return fmt.Errorf("failed to unmarshal PerformanceProfileStatus ConfigMap")
	}

	statusCopy := status.DeepCopy()

	if conditions != nil {
		statusCopy.Conditions = conditions
	}

	// check if we need to update the status
	modified := false

	// since we always set the same four conditions, we don't need to check if we need to remove old conditions
	for _, newCondition := range statusCopy.Conditions {
		oldCondition := conditionsv1.FindStatusCondition(status.Conditions, newCondition.Type)
		if oldCondition == nil {
			modified = true
			break
		}

		// ignore timestamps to avoid infinite reconcile loops
		if oldCondition.Status != newCondition.Status ||
			oldCondition.Reason != newCondition.Reason ||
			oldCondition.Message != newCondition.Message {
			modified = true
			break
		}
	}

	if statusCopy.Tuned == nil {
		tunedNamespacedname := types.NamespacedName{
			Name:      components.GetComponentName(profileName, components.ProfileNamePerformance),
			Namespace: components.NamespaceNodeTuningOperator,
		}
		tunedStatus := tunedNamespacedname.String()
		statusCopy.Tuned = &tunedStatus
		modified = true
	}

	if statusCopy.RuntimeClass == nil {
		runtimeClassName := components.GetComponentName(profileName, components.ComponentNamePrefix)
		statusCopy.RuntimeClass = &runtimeClassName
		modified = true
	}

	if !modified {
		return nil
	}
	statusBytes, err := json.Marshal(statusCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal PerformanceProfileStatus ConfigMap")
	}
	performanceProfileStatusCM.Data["status"] = string(statusBytes)

	klog.Infof("Updating the performance profile %q status", profileName)
	return w.controlPlaneClient.Update(ctx, performanceProfileStatusCM)
}

func createOrUpdatePerformanceProfileStatusConfigMap(ctx context.Context, cli client.Client, cm *corev1.ConfigMap) error {
	performanceProfileStatusConfigMapUpdateFunc := func(orig, dst *corev1.ConfigMap) {
		dst.Data["status"] = orig.Data["status"]
	}
	return handler.CreateOrUpdateConfigMap(ctx, cli, cm, performanceProfileStatusConfigMapUpdateFunc)
}

func (w *writer) encapsulateObjInConfigMap(instance *corev1.ConfigMap, status performancev2.PerformanceProfileStatus, profileName, dataKey, objectLabel string) (*corev1.ConfigMap, error) {
	encodedObj, err := json.Marshal(status)
	if err != nil {
		return nil, err
	}
	nodePoolNamespacedName, ok := instance.Annotations[operator.hypershiftNodePoolLabel]
	if !ok {
		return nil, fmt.Errorf("annotation %q not found in ConfigMap %q annotations", operator.hypershiftNodePoolLabel, client.ObjectKeyFromObject(instance).String())
	}

	name := fmt.Sprintf("%s-%s", strings.ToLower(object.GetObjectKind().GroupVersionKind().Kind), instance.Name)
	cm := handler.ConfigMapMeta(name, profileName, instance.GetNamespace(), nodePoolNamespacedName)
	err = controllerutil.SetControllerReference(instance, cm, w.scheme)
	if err != nil {
		return nil, err
	}
	cm.Labels[objectLabel] = "true"
	cm.Data = map[string]string{
		dataKey: string(encodedObj),
	}
	return cm, nil
}

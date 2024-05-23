package nodepools

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
)

func WaitForUpdatingConfig(ctx context.Context, c client.Client, NpName, namespace string) error {
	return waitForCondition(ctx, c, NpName, namespace, func(cond hypershiftv1beta1.NodePoolCondition) (done bool, err error) {
		if cond.Type == "UpdatingConfig" {
			return cond.Status == corev1.ConditionTrue, nil
		}
		return false, nil
	})
}

func WaitForConfigToBeReady(ctx context.Context, c client.Client, NpName, namespace string) error {
	return waitForCondition(ctx, c, NpName, namespace, func(cond hypershiftv1beta1.NodePoolCondition) (done bool, err error) {
		if cond.Type == "UpdatingConfig" {
			return cond.Status == corev1.ConditionFalse, nil
		}
		return false, nil
	})
}

func waitForCondition(ctx context.Context, c client.Client, NpName, namespace string, conditionFunc func(hypershiftv1beta1.NodePoolCondition) (done bool, err error)) error {
	return wait.PollUntilContextTimeout(ctx, time.Second*10, time.Minute*20, false, func(ctx context.Context) (done bool, err error) {
		np := &hypershiftv1beta1.NodePool{}
		key := client.ObjectKey{Name: NpName, Namespace: namespace}
		err = c.Get(ctx, key, np)
		if err != nil {
			return false, err
		}
		for _, cond := range np.Status.Conditions {
			return conditionFunc(cond)
		}
		return false, nil
	})
}

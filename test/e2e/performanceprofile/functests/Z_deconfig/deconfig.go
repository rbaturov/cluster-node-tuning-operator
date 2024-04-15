package Z_deconfig

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	testutils "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils"
	testclient "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/client"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/namespaces"
	nodeInspector "github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/node_inspector"
)

// This test suite is designed to perform cleanup actions that should occur after all test suites have been executed.

var _ = Describe("Deconfig", func() {
	It("Should delete the node inspector and its namespace", func() {
		err := nodeInspector.Delete(testclient.Client, testutils.NodeInspectorNamespace, testutils.NodeInspectorName)
		Expect(err).ToNot(HaveOccurred())
		err = testclient.DataPlaneClient.Delete(context.TODO(), namespaces.NodeInspectorNamespace)
		Expect(err).ToNot(HaveOccurred())
		err = namespaces.WaitForDeletion(testutils.NodeInspectorNamespace, 5*time.Minute)
		Expect(err).ToNot(HaveOccurred())
	})
})

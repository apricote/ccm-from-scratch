package e2e

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudproviderapi "k8s.io/cloud-provider/api"
	"testing"
)

func TestTaintRemoved(t *testing.T) {
	ctx := context.Background()

	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unable to list nodes: %v", err)
	}

	expectedTaint := &corev1.Taint{
		Key:    cloudproviderapi.TaintExternalCloudProvider,
		Effect: corev1.TaintEffectNoSchedule,
	}

	for _, node := range nodeList.Items {
		for _, taint := range node.Spec.Taints {
			if taint.MatchTaint(expectedTaint) {
				t.Errorf("found cloud-provider taint on node %s", node.Name)
			}
		}
	}
}

// TODO: Tests for: Labels, Shutdown, Node Removal, Addresses

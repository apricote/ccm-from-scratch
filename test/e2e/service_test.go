package e2e

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	"net"
	"net/http"
	"testing"
)

func TestServiceWorks(t *testing.T) {
	ctx := context.Background()

	_, err := client.AppsV1().Deployments(namespace).Create(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "service-nginx",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "nginx"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:stable",
							Ports: []corev1.ContainerPort{{ContainerPort: 80, Name: "web"}},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create deployment: %v", err)
	}

	svcClient := client.CoreV1().Services(namespace)

	_, err = svcClient.Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "service-nginx",
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeLoadBalancer,
			Selector: map[string]string{"app": "nginx"},
			Ports: []corev1.ServicePort{
				{
					Name:       "web",
					Protocol:   "TCP",
					Port:       80,
					TargetPort: intstr.FromString("web"),
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Wait for LB to become available
	var svc *corev1.Service
	err = retry.OnError(backoff, alwaysRetry, func() error {
		svc, err = svcClient.Get(ctx, "service-nginx", metav1.GetOptions{})
		if err != nil {
			return err
		}

		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			return fmt.Errorf("no ingress endpoints set yet")
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to wait for service to become available: %v", err)
	}

	// Send test traffic
	var ip net.IP
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		// Find public IPv4
		ip = net.ParseIP(ingress.IP).To4()
		if ip != nil && !ip.IsPrivate() {
			break
		}
	}
	if ip == nil {
		t.Fatalf("no public ipv4 available")
	}

	resp, err := http.Get(fmt.Sprintf("http://%s", ip.String()))
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}
}

// TODO: Tests for: Removing+Updating Ports, Removing Targets, Removing LBs,

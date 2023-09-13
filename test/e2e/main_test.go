package e2e

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	"os"
	"testing"
	"time"
)

var (
	client    *kubernetes.Clientset
	namespace string

	backoff = wait.Backoff{
		Steps:    4,
		Duration: 5 * time.Second,
		Factor:   5.0,
		Jitter:   0.1,
	}
	alwaysRetry = func(_ error) bool { return true }
)

func TestMain(m *testing.M) {
	err := setupKubeClient()
	if err != nil {
		panic(fmt.Errorf("unable to setup kubernetes client: %w", err).Error())
	}

	err = setupTestNamespace()
	if err != nil {
		panic(fmt.Errorf("unable to create test namespace: %w", err).Error())
	}

	exitCode := m.Run()

	err = tearDownTestNamespace()
	if err != nil {
		panic(fmt.Errorf("unable to create test namespace: %w", err).Error())
	}

	os.Exit(exitCode)
}

func setupKubeClient() error {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		return err
	}

	// create the clientset
	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	return nil
}

func setupTestNamespace() error {
	// generate random name
	namespace = fmt.Sprintf("ccm-e2e-%s", rand.String(8))

	_, err := client.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func tearDownTestNamespace() error {
	return client.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{
		PropagationPolicy: ptr.To(metav1.DeletePropagationForeground),
	})
}

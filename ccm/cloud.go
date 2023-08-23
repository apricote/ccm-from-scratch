package ccm

import (
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"io"
	cloudprovider "k8s.io/cloud-provider"
	"os"
)

const (
	providerName = "hcloud-from-scratch"
)

type CloudProvider struct {
	client *hcloud.Client
}

func (c CloudProvider) Initialize(_ cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {}

func (c CloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return LoadBalancer{client: c.client}, true
}

func (c CloudProvider) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (c CloudProvider) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return InstancesV2{client: c.client}, true
}

func (c CloudProvider) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

func (c CloudProvider) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (c CloudProvider) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

func (c CloudProvider) ProviderName() string {
	return providerName
}

func (c CloudProvider) HasClusterID() bool {
	return false
}

func newCloud(_ io.Reader) (cloudprovider.Interface, error) {
	options := []hcloud.ClientOption{
		hcloud.WithToken(os.Getenv("HCLOUD_TOKEN")),
		hcloud.WithApplication("ccm-from-scratch", ""),
	}

	if os.Getenv("HCLOUD_DEBUG") != "" {
		options = append(options, hcloud.WithDebugWriter(os.Stderr))
	}

	client := hcloud.NewClient(options...)

	return CloudProvider{client: client}, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(providerName, newCloud)
}

package ccm

import (
	"io"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	providerName = "hcloud-from-scratch"
)

type CloudProvider struct{}

func (c CloudProvider) Initialize(_ cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {}

func (c CloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

func (c CloudProvider) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (c CloudProvider) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return InstancesV2{}, true
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
	return CloudProvider{}, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(providerName, newCloud)
}

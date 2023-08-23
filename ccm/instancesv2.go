package ccm

import (
	"context"
	"fmt"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

type InstancesV2 struct {
	client *hcloud.Client
}

func (i InstancesV2) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	return true, nil
}

func (i InstancesV2) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	return false, nil
}

func (i InstancesV2) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	server, _, err := i.client.Server.GetByName(ctx, node.Name)
	if err != nil {
		return nil, err // TODO: wrap it
	}
	if server == nil {
		return nil, fmt.Errorf("server not found")
	}

	klog.Warningf("Server: %+v", server)

	return &cloudprovider.InstanceMetadata{
		ProviderID:    fmt.Sprintf("%s://%d", providerName, server.ID),
		InstanceType:  server.ServerType.Name,
		NodeAddresses: getNodeAddresses(server),
		Zone:          server.Datacenter.Location.Name,
		Region:        string(server.Datacenter.Location.NetworkZone),
	}, nil
}

func getNodeAddresses(server *hcloud.Server) []v1.NodeAddress {
	addresses := []v1.NodeAddress{
		{
			Type:    v1.NodeHostName,
			Address: server.Name,
		},
	}

	if !server.PublicNet.IPv4.IsUnspecified() {
		addresses = append(addresses, v1.NodeAddress{
			Type:    v1.NodeExternalIP,
			Address: server.PublicNet.IPv4.IP.String(),
		})
	}

	if !server.PublicNet.IPv6.IsUnspecified() {
		addresses = append(addresses, v1.NodeAddress{
			Type:    v1.NodeExternalIP,
			Address: server.PublicNet.IPv6.IP.String(),
		})
	}

	return addresses
}

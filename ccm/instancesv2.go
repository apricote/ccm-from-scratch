package ccm

import (
	"context"
	"fmt"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
	"strconv"
	"strings"
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
	server, err := i.getServerForNode(ctx, node)
	if err != nil {
		return nil, err
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

func (i InstancesV2) getServerForNode(ctx context.Context, node *v1.Node) (*hcloud.Server, error) {
	var server *hcloud.Server

	if node.Spec.ProviderID != "" {
		providerID, found := strings.CutPrefix(node.Spec.ProviderID, fmt.Sprintf("%s://", providerName))
		if !found {
			return nil, fmt.Errorf("ProviderID does not follow expected format: %s", node.Spec.ProviderID)
		}

		id, err := strconv.ParseInt(providerID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unable to parse ProviderID to integer: %w", err)
		}

		server, _, err = i.client.Server.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		server, _, err = i.client.Server.GetByName(ctx, node.Name)
		if err != nil {
			return nil, err
		}
	}

	if server == nil {
		return nil, fmt.Errorf("server not found")
	}

	return server, nil
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

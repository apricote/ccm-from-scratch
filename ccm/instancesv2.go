package ccm

import (
	"context"
	"errors"
	"fmt"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	"strconv"
	"strings"
)

var (
	errServerNotFound = errors.New("server not found")
)

var _ cloudprovider.InstancesV2 = InstancesV2{}

type InstancesV2 struct {
	client    *hcloud.Client
	networkID int64
}

func (i InstancesV2) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	_, err := i.getServerForNode(ctx, node)
	if err != nil {
		if errors.Is(err, errServerNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (i InstancesV2) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	server, err := i.getServerForNode(ctx, node)
	if err != nil {
		return false, err
	}

	return server.Status != hcloud.ServerStatusRunning, nil
}

func (i InstancesV2) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	server, err := i.getServerForNode(ctx, node)
	if err != nil {
		return nil, err
	}

	return &cloudprovider.InstanceMetadata{
		ProviderID:    fmt.Sprintf("%s://%d", providerName, server.ID),
		InstanceType:  server.ServerType.Name,
		NodeAddresses: getNodeAddresses(server, i.networkID),
		Zone:          server.Datacenter.Location.Name,
		Region:        string(server.Datacenter.Location.NetworkZone),
	}, nil
}

func (i InstancesV2) getServerForNode(ctx context.Context, node *v1.Node) (*hcloud.Server, error) {
	var server *hcloud.Server

	if node.Spec.ProviderID != "" {
		id, err := getProviderID(node)
		if err != nil {
			return nil, err
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
		return nil, errServerNotFound
	}

	return server, nil
}

func getProviderID(node *v1.Node) (int64, error) {
	providerID, found := strings.CutPrefix(node.Spec.ProviderID, fmt.Sprintf("%s://", providerName))
	if !found {
		return 0, fmt.Errorf("ProviderID does not follow expected format: %s", node.Spec.ProviderID)
	}

	id, err := strconv.ParseInt(providerID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unable to parse ProviderID to integer: %w", err)
	}
	return id, nil
}

func getNodeAddresses(server *hcloud.Server, networkID int64) []v1.NodeAddress {
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

	for _, privNet := range server.PrivateNet {
		if privNet.Network.ID != networkID {
			continue
		}

		if !privNet.IP.IsUnspecified() {
			addresses = append(addresses, v1.NodeAddress{
				Type:    v1.NodeInternalIP,
				Address: privNet.IP.String(),
			})
		}
	}

	return addresses
}

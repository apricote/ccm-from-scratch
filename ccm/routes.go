package ccm

import (
	"context"
	"fmt"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"net"
)

type Routes struct {
	client    *hcloud.Client
	networkID int64

	// Caches, TODO: Locking

	network *hcloud.Network
	servers []*hcloud.Server
}

func (r *Routes) ListRoutes(ctx context.Context, _ string) ([]*cloudprovider.Route, error) {
	err := r.updateNetwork(ctx)
	if err != nil {
		return nil, err
	}

	err = r.updateServers(ctx, err)
	if err != nil {
		return nil, err
	}

	routes := make([]*cloudprovider.Route, 0, len(r.network.Routes))

	for _, hcloudRoute := range r.network.Routes {
		route := &cloudprovider.Route{
			Name:            fmt.Sprintf("%s-%s", hcloudRoute.Destination.String(), hcloudRoute.Gateway.String()),
			DestinationCIDR: hcloudRoute.Destination.String(),
		}

		targetNode := findTargetNode(r.network, hcloudRoute.Gateway, r.servers)
		if targetNode == nil {
			route.Blackhole = true
		} else {
			route.TargetNode = types.NodeName(targetNode.Name)
		}

		routes = append(routes, route)
	}

	return routes, nil
}

func (r *Routes) updateServers(ctx context.Context, err error) error {
	servers, err := r.client.Server.All(ctx)
	if err != nil {
		return err
	}
	//
	r.servers = servers
	return nil
}

// updateNetwork changes the semantics of GetByID to also return an error if no network was found.
func (r *Routes) updateNetwork(ctx context.Context) error {
	network, _, err := r.client.Network.GetByID(ctx, r.networkID)
	if err != nil {
		return err
	}

	if network == nil {
		return fmt.Errorf("network not found: ID=%d", r.networkID)
	}

	r.network = network
	return nil
}

func findTargetNode(network *hcloud.Network, gatewayIP net.IP, servers []*hcloud.Server) *hcloud.Server {
	for _, server := range servers {
		for _, serverNetwork := range server.PrivateNet {
			if serverNetwork.Network.ID != network.ID {
				continue
			}

			if serverNetwork.IP.Equal(gatewayIP) {
				// Target Node
				return server
			}
		}
	}

	return nil
}

func (r *Routes) CreateRoute(ctx context.Context, _ string, _ string, route *cloudprovider.Route) error {
	hcloudRoute, err := r.k8sRouteToHCloudRoute(route)
	if err != nil {
		return err
	}

	// TODO: Check for existing route (can happen in case Server was deleted in API but Node still exists in Kubernetes)
	action, _, err := r.client.Network.AddRoute(ctx, r.network, hcloud.NetworkAddRouteOpts{Route: *hcloudRoute})
	if err != nil {
		return fmt.Errorf("unable to create route: unable to create route in hcloud: %w", err)
	}

	_, errCh := r.client.Action.WatchProgress(ctx, action)
	if err = <-errCh; err != nil {
		return fmt.Errorf("unable to create route: creating route action failed: %w", err)
	}

	return nil
}

func (r *Routes) DeleteRoute(ctx context.Context, _ string, route *cloudprovider.Route) error {
	hcloudRoute, err := r.k8sRouteToHCloudRoute(route)
	if err != nil {
		return err
	}

	action, _, err := r.client.Network.DeleteRoute(ctx, r.network, hcloud.NetworkDeleteRouteOpts{Route: *hcloudRoute})
	if err != nil {
		return fmt.Errorf("unable to delete route: unable to delete route in hcloud: %w", err)
	}

	_, errCh := r.client.Action.WatchProgress(ctx, action)
	if err = <-errCh; err != nil {
		return fmt.Errorf("unable to delete route: deleting route action failed: %w", err)
	}

	return nil
}

func (r *Routes) k8sRouteToHCloudRoute(route *cloudprovider.Route) (*hcloud.NetworkRoute, error) {
	_, destinationCIDR, err := net.ParseCIDR(route.DestinationCIDR)
	if err != nil {
		return nil, fmt.Errorf("unable to parse destination cidr: %w", err)
	}

	var gatewayIP net.IP
	for _, address := range route.TargetNodeAddresses {
		if address.Type == v1.NodeInternalIP {
			gatewayIP = net.ParseIP(address.Address)
			if gatewayIP == nil {
				return nil, fmt.Errorf("unable to parse node address: %s", address.Address)
			}
			break
		}
	}
	if gatewayIP == nil {
		// Fallback if no internal ip is set
		for _, hcloudRoute := range r.network.Routes {
			if hcloudRoute.Destination.String() == route.DestinationCIDR {
				gatewayIP = hcloudRoute.Gateway
			}
		}
		if gatewayIP == nil {
			return nil, fmt.Errorf("unable to figure out gateway ip: neither NodeInternalIP is set, nor does a matching route exist in in hcloud")
		}
	}

	return &hcloud.NetworkRoute{
		Destination: destinationCIDR,
		Gateway:     gatewayIP,
	}, nil
}

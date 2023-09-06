package ccm

import (
	"context"
	"fmt"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	v1 "k8s.io/api/core/v1"
)

type LoadBalancer struct {
	client *hcloud.Client
}

func (l LoadBalancer) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	lbName := getLBName(clusterName, service)
	lb, _, err := l.client.LoadBalancer.GetByName(ctx, lbName)
	if err != nil {
		return nil, false, fmt.Errorf("unable to check for existing loadbalancer: %w", err)
	}

	if lb == nil {
		return nil, false, nil
	}

	return getLBStatus(lb), true, nil
}

func (l LoadBalancer) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	return getLBName(clusterName, service)
}

func (l LoadBalancer) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	// Get existing LoadBalancer
	lbName := getLBName(clusterName, service)
	lb, _, err := l.client.LoadBalancer.GetByName(ctx, lbName)
	if err != nil {
		return nil, fmt.Errorf("unable to check for existing loadbalancer: %w", err)
	}

	if lb == nil {
		// (If none) create new LoadBalancer
		result, _, err := l.client.LoadBalancer.Create(ctx, hcloud.LoadBalancerCreateOpts{
			Name:             lbName,
			LoadBalancerType: &hcloud.LoadBalancerType{ID: 1, Name: "lb11"},
			Location:         &hcloud.Location{ID: 1, Name: "fsn1"},
		})
		if err != nil {
			return nil, fmt.Errorf("unable to create new loadbalancer: %w", err)
		}

		// Wait for IP to be set
		_, errCh := l.client.Action.WatchProgress(ctx, result.Action)
		err = <-errCh
		if err != nil {
			return nil, fmt.Errorf("unable to start new loadbalancer: %w", err)
		}

		lb, _, err = l.client.LoadBalancer.GetByID(ctx, result.LoadBalancer.ID)
		if err != nil {
			return nil, fmt.Errorf("unable to refresh new loadbalancer: %w", err)
		}
	}

	// Check the services
	// Create missing hcloud lb services
	for _, port := range service.Spec.Ports {
		foundExistingService := false
		for _, svc := range lb.Services {
			if svc.ListenPort == int(port.Port) {
				foundExistingService = true
				// Match
				if svc.DestinationPort != int(port.NodePort) {
					_, _, err = l.client.LoadBalancer.UpdateService(ctx, lb, svc.ListenPort, hcloud.LoadBalancerUpdateServiceOpts{
						Protocol:        hcloud.LoadBalancerServiceProtocolTCP,
						DestinationPort: hcloud.Ptr(int(port.NodePort)),
					})
					if err != nil {
						return nil, err
					}
				}
				break
			}
		}

		if !foundExistingService {
			// No existing service found
			_, _, err = l.client.LoadBalancer.AddService(ctx, lb, hcloud.LoadBalancerAddServiceOpts{
				Protocol:        hcloud.LoadBalancerServiceProtocolTCP,
				ListenPort:      hcloud.Ptr(int(port.Port)),
				DestinationPort: hcloud.Ptr(int(port.NodePort)),
			})
			if err != nil {
				return nil, err
			}
		}

	}

	// TODO: Clean up LB Services that do not belong to any service ports.

	status, err := l.updateLBTargets(ctx, nodes, lb)
	if err != nil {
		return status, err
	}

	return getLBStatus(lb), nil
}

func (l LoadBalancer) updateLBTargets(ctx context.Context, nodes []*v1.Node, lb *hcloud.LoadBalancer) (*v1.LoadBalancerStatus, error) {
	// Check the targets
	// Create missing hcloud lb targets
	for _, node := range nodes {
		foundExistingTarget := false

		providerID, err := getProviderID(node)
		if err != nil {
			return nil, err
		}

		for _, target := range lb.Targets {
			if target.Server.Server.ID == providerID {
				foundExistingTarget = true
				break
			}
		}

		if !foundExistingTarget {
			_, _, err = l.client.LoadBalancer.AddServerTarget(ctx, lb, hcloud.LoadBalancerAddServerTargetOpts{
				Server: &hcloud.Server{
					ID: providerID,
				},
			})
			if err != nil {
				return nil, err
			}
		}
	}

	// TODO: Clean up LB Targets that do not belong to any nodes targeted.
	return nil, nil
}

func getLBStatus(lb *hcloud.LoadBalancer) *v1.LoadBalancerStatus {
	lbStatus := &v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{}}

	if lb.PublicNet.IPv4.IP != nil && !lb.PublicNet.IPv4.IP.IsUnspecified() {
		lbStatus.Ingress = append(lbStatus.Ingress, v1.LoadBalancerIngress{
			IP:       lb.PublicNet.IPv4.IP.String(),
			Hostname: lb.PublicNet.IPv4.DNSPtr,
		})
	}

	if lb.PublicNet.IPv6.IP != nil && !lb.PublicNet.IPv6.IP.IsUnspecified() {
		lbStatus.Ingress = append(lbStatus.Ingress, v1.LoadBalancerIngress{
			IP:       lb.PublicNet.IPv6.IP.String(),
			Hostname: lb.PublicNet.IPv6.DNSPtr,
		})
	}

	return lbStatus
}

func getLBName(clusterName string, service *v1.Service) string {
	return fmt.Sprintf("%s-%s-%s", clusterName, service.Namespace, service.Name)
}

func (l LoadBalancer) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	// Get existing LoadBalancer
	lbName := getLBName(clusterName, service)
	lb, _, err := l.client.LoadBalancer.GetByName(ctx, lbName)
	if err != nil {
		return fmt.Errorf("unable to check for existing loadbalancer: %w", err)
	}

	if lb == nil {
		return fmt.Errorf("no existing loadbalancer found")
	}

	if _, err = l.updateLBTargets(ctx, nodes, lb); err != nil {
		return err
	}
	return nil
}

func (l LoadBalancer) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	// Get existing LoadBalancer
	lbName := getLBName(clusterName, service)
	lb, _, err := l.client.LoadBalancer.GetByName(ctx, lbName)
	if err != nil {
		return fmt.Errorf("unable to check for existing loadbalancer: %w", err)
	}

	if lb == nil {
		return nil
	}

	_, err = l.client.LoadBalancer.Delete(ctx, lb)
	if err != nil {
		return err
	}

	return nil
}

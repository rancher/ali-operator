package alibaba

import (
	"context"
	"fmt"
	"strconv"

	cs "github.com/alibabacloud-go/cs-20151215/v5/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/rancher/ali-operator/pkg/alibaba/services"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
)

func Create(ctx context.Context, client services.ClustersClientInterface, configSpec *aliv1.AliClusterConfigSpec) error {
	err := validateCreateRequest(configSpec)
	if err != nil {
		return err
	}

	request := newClusterCreateRequest(configSpec)
	resp, err := client.CreateCluster(ctx, request)
	if err != nil {
		return err
	}

	configSpec.ClusterID = *resp.Body.ClusterId
	return nil
}

func validateCreateRequest(configSpec *aliv1.AliClusterConfigSpec) error {
	if configSpec.ClusterName == "" {
		return fmt.Errorf("cluster display name is required")
	} else if configSpec.RegionID == "" {
		return fmt.Errorf("region id is required")
	}

	return nil
}

// newClusterCreateRequest creates a CreateClusterRequest that can be submitted to ACK
func newClusterCreateRequest(configSpec *aliv1.AliClusterConfigSpec) *cs.CreateClusterRequest {
	req := &cs.CreateClusterRequest{}

	req.Name = tea.String(configSpec.ClusterName)
	req.ClusterType = tea.String(configSpec.ClusterType)
	req.ClusterSpec = tea.String(configSpec.ClusterSpec)
	req.RegionId = tea.String(configSpec.RegionID)
	req.KubernetesVersion = tea.String(configSpec.KubernetesVersion)
	req.Vpcid = tea.String(configSpec.VpcID)
	req.ContainerCidr = tea.String(configSpec.ContainerCIDR)
	req.ServiceCidr = tea.String(configSpec.ServiceCIDR)
	req.NodeCidrMask = tea.String(strconv.Itoa(int(configSpec.NodeCIDRMask)))
	req.SnatEntry = tea.Bool(configSpec.SNATEntry)
	req.ProxyMode = tea.String(configSpec.ProxyMode)
	req.EndpointPublicAccess = tea.Bool(configSpec.EndpointPublicAccess)
	req.SecurityGroupId = tea.String(configSpec.SecurityGroupID)
	req.SshFlags = tea.Bool(configSpec.SSHFlags)
	req.Addons = ConvertAddons(configSpec)
	req.PodVswitchIds = tea.StringSlice(configSpec.PodVswitchIDs)
	req.ResourceGroupId = tea.String(configSpec.ResourceGroupID)
	req.VswitchIds = tea.StringSlice(configSpec.VSwitchIDs)
	return req
}

func ConvertAddons(configSpec *aliv1.AliClusterConfigSpec) []*cs.Addon {
	if configSpec == nil || len(configSpec.Addons) == 0 {
		// flannel
		return nil
	}

	addons := make([]*cs.Addon, len(configSpec.Addons))
	for i, addon := range configSpec.Addons {
		addons[i] = &cs.Addon{
			Name:   &addon.Name,
			Config: &addon.Config,
		}
	}

	return addons
}

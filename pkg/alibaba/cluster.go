package alibaba

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/rancher/ali-operator/pkg/alibaba/services"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	cs "github.com/rancher/muchang/cs/client"
	"github.com/rancher/muchang/utils/tea"
	"github.com/sirupsen/logrus"
)

func Create(ctx context.Context, client services.ClustersClientInterface, configSpec *aliv1.AliClusterConfigSpec) (string, error) {
	err := validateCreateRequest(configSpec)
	if err != nil {
		return "", err
	}

	request := newClusterCreateRequest(configSpec)
	logrus.Infof("Creating cluster %s", configSpec.ClusterName)
	resp, err := client.CreateCluster(ctx, request)
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Body == nil {
		return "", errors.New("received invalid cluster response")
	}

	return *resp.Body.ClusterId, nil
}

func validateCreateRequest(configSpec *aliv1.AliClusterConfigSpec) error {
	if configSpec.ClusterName == "" {
		return ErrRequiredClusterName
	} else if configSpec.RegionID == "" {
		return ErrRequiredRegionID
	}
	if configSpec.ClusterType != ManagedClusterType {
		return fmt.Errorf(ErrInvalidClusterType, configSpec.ClusterType)
	}

	if len(configSpec.ZoneIDs) == 0 {
		if configSpec.VpcID == "" {
			return errors.New("vpcId is required if zoneIds are not provided")
		} else if len(configSpec.VSwitchIDs) == 0 {
			return errors.New("vSwitchIds are required if zoneIds are not provided")
		}
	} else if configSpec.VpcID != "" || len(configSpec.VSwitchIDs) != 0 {
		return errors.New("zoneIds should not be used together with vpcId and vSwitchIds")
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
	req.NodeCidrMask = tea.String(strconv.Itoa(configSpec.NodeCIDRMask))
	req.SnatEntry = tea.Bool(configSpec.SNATEntry)
	req.ProxyMode = tea.String(configSpec.ProxyMode)
	req.EndpointPublicAccess = tea.Bool(configSpec.EndpointPublicAccess)
	req.SecurityGroupId = tea.String(configSpec.SecurityGroupID)
	req.Addons = GetAddons(configSpec)
	req.PodVswitchIds = tea.StringSlice(configSpec.PodVswitchIDs)
	req.ResourceGroupId = tea.String(configSpec.ResourceGroupID)
	req.VswitchIds = tea.StringSlice(configSpec.VSwitchIDs)
	req.IsEnterpriseSecurityGroup = configSpec.IsEnterpriseSecurityGroup
	return req
}

func GetAddons(configSpec *aliv1.AliClusterConfigSpec) []*cs.Addon {
	if configSpec == nil || len(configSpec.Addons) == 0 {
		// flannel
		return nil
	}

	addons := make([]*cs.Addon, len(configSpec.Addons))
	for i, addon := range configSpec.Addons {
		currentAddon := addon
		addons[i] = &cs.Addon{
			Name:   &currentAddon.Name,
			Config: &currentAddon.Config,
		}
	}

	return addons
}

func GetCluster(ctx context.Context, clustersClient services.ClustersClientInterface, clusterID string) (*cs.DescribeClusterDetailResponseBody, error) {
	clusterResp, err := clustersClient.DescribeClusterDetail(ctx, &clusterID)
	if err != nil {
		return nil, err
	}
	if clusterResp == nil || clusterResp.Body == nil {
		return nil, fmt.Errorf("the cluster is nil, indicating no cluster information is available")
	}
	return clusterResp.Body, nil
}

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/rancher/ali-operator/pkg/alibaba"
	"github.com/rancher/ali-operator/pkg/alibaba/services"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	cs "github.com/rancher/muchang/cs/client"
	"github.com/rancher/muchang/utils/tea"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
)

func BuildUpstreamClusterState(secretsCache wranglerv1.SecretCache, configSpec *aliv1.AliClusterConfigSpec) (*aliv1.AliClusterConfigSpec, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if configSpec == nil {
		logrus.Warn("Warning BuildUpstreamClusterState: The 'configSpec' data is nil, the cluster's configSpec is not available")
		return configSpec, nil
	}
	credentials, err := alibaba.GetSecrets(secretsCache, configSpec)
	if err != nil {
		return nil, fmt.Errorf("error getting credentials: %w", err)
	}
	clustersClient, err := services.NewClustersClient(credentials, configSpec.RegionID)
	if err != nil {
		return nil, fmt.Errorf("error creating client secret credential: %w", err)
	}

	clusterResp, err := clustersClient.DescribeClusterDetail(ctx, &configSpec.ClusterID)
	if err != nil {
		return configSpec, err
	}

	if clusterResp == nil || clusterResp.Body == nil {
		return configSpec, errors.New("received empty cluster response")
	}

	cluster := clusterResp.Body

	endpointPublicAccess := configSpec.EndpointPublicAccess
	masterURL := tea.StringValue(cluster.MasterUrl)
	if masterURL != "" {
		masterURLConfig := map[string]interface{}{}
		err = json.Unmarshal([]byte(masterURL), &masterURLConfig)
		if err != nil {
			return nil, fmt.Errorf("error parsing master_url of cluster: %v", err)
		}
		if _, ok := masterURLConfig["api_server_endpoint"]; ok {
			endpointPublicAccess = true
		} else {
			endpointPublicAccess = false
		}
	}

	newSpec := &aliv1.AliClusterConfigSpec{
		ClusterName:          tea.StringValue(cluster.Name),
		ClusterID:            tea.StringValue(cluster.ClusterId),
		ClusterType:          tea.StringValue(cluster.ClusterType),
		ClusterSpec:          tea.StringValue(cluster.ClusterSpec),
		KubernetesVersion:    tea.StringValue(cluster.CurrentVersion),
		RegionID:             tea.StringValue(cluster.RegionId),
		VpcID:                tea.StringValue(cluster.VpcId),
		VSwitchIDs:           tea.StringSliceValue(cluster.VswitchIds),
		ContainerCIDR:        tea.StringValue(cluster.ContainerCidr),
		ServiceCIDR:          tea.StringValue(cluster.ServiceCidr),
		EndpointPublicAccess: endpointPublicAccess,
		ProxyMode:            tea.StringValue(cluster.ProxyMode),
		SecurityGroupID:      tea.StringValue(cluster.SecurityGroupId),
		ResourceGroupID:      tea.StringValue(cluster.ResourceGroupId),
		// setting the fields from config spec which can only be set while creation to make things consistent.
		ZoneIDs:                   configSpec.ZoneIDs,
		PodVswitchIDs:             configSpec.PodVswitchIDs,
		IsEnterpriseSecurityGroup: configSpec.IsEnterpriseSecurityGroup,
		SNATEntry:                 configSpec.SNATEntry,
	}

	nodeCIDRMask := tea.StringValue(cluster.NodeCidrMask)
	if nodeCIDRMask != "" {
		nodeCIDRMaskVal, err := strconv.Atoi(nodeCIDRMask)
		if err != nil {
			logrus.Warnf("error parsing nodeCIDRMask value:%v", err)
		} else {
			newSpec.NodeCIDRMask = nodeCIDRMaskVal
		}
	}

	nodePools, err := alibaba.GetNodePools(ctx, clustersClient, configSpec)
	if err != nil {
		if errors.Is(err, alibaba.ErrEmptyClusterNodePools) {
			return configSpec, nil
		}
		return configSpec, err
	}
	if len(nodePools) > 0 {
		newSpec.NodePools = alibaba.ToNodePoolConfig(nodePools)
	}
	return newSpec, nil
}

func GetUserConfig(secretsCache wranglerv1.SecretCache, configSpec *aliv1.AliClusterConfigSpec) (*cs.DescribeClusterUserKubeconfigResponseBody, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	credentials, err := alibaba.GetSecrets(secretsCache, configSpec)
	if err != nil {
		return nil, fmt.Errorf("error getting credentials: %w", err)
	}
	clustersClient, err := services.NewClustersClient(credentials, configSpec.RegionID)
	if err != nil {
		return nil, fmt.Errorf("error creating client secret credential: %w", err)
	}
	kubeConfigResp, err := clustersClient.DescribeClusterUserKubeconfig(ctx, &configSpec.ClusterID, &cs.DescribeClusterUserKubeconfigRequest{})
	if err != nil {
		return nil, err
	}

	return kubeConfigResp.Body, nil
}

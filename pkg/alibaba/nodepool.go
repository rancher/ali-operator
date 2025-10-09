package alibaba

import (
	"context"
	"errors"
	"fmt"

	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	cs "github.com/rancher/muchang/cs/client"
	"github.com/sirupsen/logrus"

	"github.com/rancher/ali-operator/pkg/alibaba/services"
	"github.com/rancher/muchang/utils/tea"
)

func GetNodePools(ctx context.Context, client services.ClustersClientInterface, configSpec *aliv1.AliClusterConfigSpec) ([]*cs.DescribeClusterNodePoolsResponseBodyNodepools, error) {
	nodePoolsResp, err := client.DescribeClusterNodePools(ctx, &configSpec.ClusterID, &cs.DescribeClusterNodePoolsRequest{})
	if err != nil {
		return nil, err
	}
	if nodePoolsResp == nil || nodePoolsResp.Body == nil {
		return nil, ErrEmptyClusterNodePools
	}

	return nodePoolsResp.Body.Nodepools, nil
}

func ToNodePoolConfig(nodePools []*cs.DescribeClusterNodePoolsResponseBodyNodepools) []aliv1.AliNodePool {
	var nodePoolList []aliv1.AliNodePool
	for _, nodePool := range nodePools {
		var dataDisks []aliv1.AliDisk
		if nodePool.ScalingGroup.DataDisks != nil {
			for _, disk := range nodePool.ScalingGroup.DataDisks {
				if disk != nil {
					dataDisks = append(dataDisks, aliv1.AliDisk{
						Category:             tea.StringValue(disk.Category),
						Size:                 tea.Int64Value(disk.Size),
						Encrypted:            tea.StringValue(disk.Encrypted),
						AutoSnapshotPolicyID: tea.StringValue(disk.AutoSnapshotPolicyId),
					})
				}
			}
		}
		np := aliv1.AliNodePool{
			DataDisks: dataDisks,
		}
		if nodePool.NodepoolInfo != nil {
			np.NodePoolID = tea.StringValue(nodePool.NodepoolInfo.NodepoolId)
			np.Name = tea.StringValue(nodePool.NodepoolInfo.Name)
		}
		if nodePool.ScalingGroup != nil {
			np.DesiredSize = nodePool.ScalingGroup.DesiredSize
			np.AutoRenew = tea.BoolValue(nodePool.ScalingGroup.AutoRenew)
			np.AutoRenewPeriod = tea.Int64Value(nodePool.ScalingGroup.AutoRenewPeriod)
			np.InstanceChargeType = tea.StringValue(nodePool.ScalingGroup.InstanceChargeType)
			np.InstanceTypes = tea.StringSliceValue(nodePool.ScalingGroup.InstanceTypes)
			np.KeyPair = tea.StringValue(nodePool.ScalingGroup.KeyPair)
			np.Period = tea.Int64Value(nodePool.ScalingGroup.Period)
			np.PeriodUnit = tea.StringValue(nodePool.ScalingGroup.PeriodUnit)
			np.ImageType = tea.StringValue(nodePool.ScalingGroup.ImageType)
			np.SystemDiskCategory = tea.StringValue(nodePool.ScalingGroup.SystemDiskCategory)
			np.SystemDiskSize = tea.Int64Value(nodePool.ScalingGroup.SystemDiskSize)
			np.VSwitchIDs = tea.StringSliceValue(nodePool.ScalingGroup.VswitchIds)
		}
		if nodePool.AutoScaling != nil {
			np.ScalingType = tea.StringValue(nodePool.AutoScaling.Type)
			np.EnableAutoScaling = nodePool.AutoScaling.Enable
			np.MaxInstances = nodePool.AutoScaling.MaxInstances
			np.MinInstances = nodePool.AutoScaling.MinInstances
		}
		if nodePool.KubernetesConfig != nil {
			np.Runtime = tea.StringValue(nodePool.KubernetesConfig.Runtime)
			np.RuntimeVersion = tea.StringValue(nodePool.KubernetesConfig.RuntimeVersion)
		}
		nodePoolList = append(nodePoolList, np)
	}
	return nodePoolList
}

func CreateNodePool(ctx context.Context, client services.ClustersClientInterface, configSpec *aliv1.AliClusterConfigSpec, npConfig *aliv1.AliNodePool) (*cs.CreateClusterNodePoolResponseBody, error) {
	req := newNodePoolCreateRequest(npConfig, &configSpec.ResourceGroupID, configSpec.VSwitchIDs)
	resp, err := client.CreateClusterNodePool(ctx, &configSpec.ClusterID, req)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Body == nil {
		return nil, errors.New("received empty response")
	}

	return resp.Body, nil
}

func newNodePoolCreateRequest(npConfig *aliv1.AliNodePool, resourceGroupID *string, clusterVSwitchIDs []string) *cs.CreateClusterNodePoolRequest {
	var dataDiskList []*cs.DataDisk
	for _, dataDisk := range npConfig.DataDisks {
		dataDiskList = append(dataDiskList, &cs.DataDisk{
			Category:             tea.String(dataDisk.Category),
			Size:                 tea.Int64(dataDisk.Size),
			Encrypted:            tea.String(dataDisk.Encrypted),
			AutoSnapshotPolicyId: tea.String(dataDisk.AutoSnapshotPolicyID),
		})
	}

	vSwitchIDs := npConfig.VSwitchIDs
	if len(npConfig.VSwitchIDs) == 0 {
		vSwitchIDs = clusterVSwitchIDs
	}

	return &cs.CreateClusterNodePoolRequest{
		AutoScaling: &cs.CreateClusterNodePoolRequestAutoScaling{
			Enable:       npConfig.EnableAutoScaling,
			MaxInstances: npConfig.MaxInstances,
			MinInstances: npConfig.MinInstances,
			Type:         tea.String(npConfig.ScalingType),
		},
		NodepoolInfo: &cs.CreateClusterNodePoolRequestNodepoolInfo{
			Name:            tea.String(npConfig.Name),
			ResourceGroupId: resourceGroupID,
		},
		KubernetesConfig: &cs.CreateClusterNodePoolRequestKubernetesConfig{
			Runtime:        tea.String(npConfig.Runtime),
			RuntimeVersion: tea.String(npConfig.RuntimeVersion),
		},
		ScalingGroup: &cs.CreateClusterNodePoolRequestScalingGroup{
			AutoRenew:          tea.Bool(npConfig.AutoRenew),
			AutoRenewPeriod:    tea.Int64(npConfig.AutoRenewPeriod),
			DataDisks:          dataDiskList,
			InstanceChargeType: tea.String(npConfig.InstanceChargeType),
			InstanceTypes:      tea.StringSlice(npConfig.InstanceTypes),
			KeyPair:            tea.String(npConfig.KeyPair),
			Period:             tea.Int64(npConfig.Period),
			PeriodUnit:         tea.String(npConfig.PeriodUnit),
			ImageType:          tea.String(npConfig.ImageType),
			ImageId:            tea.String(npConfig.ImageID),
			SystemDiskCategory: tea.String(npConfig.SystemDiskCategory),
			SystemDiskSize:     tea.Int64(npConfig.SystemDiskSize),
			VswitchIds:         tea.StringSlice(vSwitchIDs),
			DesiredSize:        npConfig.DesiredSize,
		},
	}
}

func DeleteNodePool(ctx context.Context, client services.ClustersClientInterface, clusterID string, nodePool *aliv1.AliNodePool) error {
	var req *cs.ModifyClusterNodePoolRequest
	if tea.Int64Value(nodePool.DesiredSize) == 0 {
		req = nil
	}

	if nodePool.DesiredSize != nil && tea.Int64Value(nodePool.DesiredSize) != 0 {
		req = &cs.ModifyClusterNodePoolRequest{
			ScalingGroup: &cs.ModifyClusterNodePoolRequestScalingGroup{
				DesiredSize: tea.Int64(0),
			},
		}
	} else if nodePool.MinInstances != nil && nodePool.MaxInstances != nil && tea.Int64Value(nodePool.MinInstances) != 0 || tea.Int64Value(nodePool.MaxInstances) != 0 {
		req = &cs.ModifyClusterNodePoolRequest{
			AutoScaling: &cs.ModifyClusterNodePoolRequestAutoScaling{
				Enable:       tea.Bool(true),
				MaxInstances: tea.Int64(0),
				MinInstances: tea.Int64(0),
			},
		}
	}

	if req != nil {
		_, err := client.ModifyClusterNodePool(ctx, &clusterID, &nodePool.NodePoolID, req)
		if err != nil {
			return fmt.Errorf("failed to update nodepool %s size to 0: %w", nodePool.NodePoolID, err)
		}

		return nil
	}

	nodesResp, err := GetClusterNodes(ctx, client, clusterID, nodePool.NodePoolID)
	if err != nil {
		return fmt.Errorf("failed to get nodes for nodepool %s: %w", nodePool.NodePoolID, err)
	}
	if len(nodesResp.Nodes) > 0 {
		var instanceIDs []string
		for _, node := range nodesResp.Nodes {
			if node.InstanceId != nil {
				instanceIDs = append(instanceIDs, *node.InstanceId)
			}
		}
		if err := ScaleDownNodePool(ctx, client, clusterID, nodePool.NodePoolID, instanceIDs); err != nil {
			return fmt.Errorf("failed to scale down nodes for nodepool %s: %w", nodePool.NodePoolID, err)
		}
		return nil
	}

	_, err = client.DeleteClusterNodePool(ctx, &clusterID, &nodePool.NodePoolID, &cs.DeleteClusterNodepoolRequest{})
	if err != nil {
		return fmt.Errorf("failed to delete nodepool %s: %w", nodePool.NodePoolID, err)
	}
	logrus.Infof("Nodepool [%s] (id %s) deleted from cluster [%s]", nodePool.Name, nodePool.NodePoolID, clusterID)
	return nil
}

func ScaleDownNodePool(ctx context.Context, client services.ClustersClientInterface, clusterID, npID string, instanceIDs []string) error {
	req := &cs.RemoveNodePoolNodesRequest{
		InstanceIds: tea.StringSlice(instanceIDs),
	}
	_, err := client.RemoveNodePoolNodes(ctx, &clusterID, &npID, req)
	if err != nil {
		return err
	}
	return nil
}

func GetClusterNodes(ctx context.Context, client services.ClustersClientInterface, clusterID string, npID string) (*cs.DescribeClusterNodesResponseBody, error) {
	req := &cs.DescribeClusterNodesRequest{
		NodepoolId: &npID,
	}
	resp, err := client.DescribeClusterNodes(ctx, &clusterID, req)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Body == nil {
		return nil, errors.New("received empty response")
	}

	return resp.Body, nil
}

func WaitForNodePool(state *string) bool {
	if state == nil {
		return false
	}
	return *state == NodePoolStatusScaling || *state == NodePoolStatusDeleting || *state == NodePoolStatusInitial ||
		*state == NodePoolStatusUpdating || *state == NodePoolStatusRemoving || *state == NodePoolStatusRemovingNodes
}

func UpdateNodePoolAutoScalingConfig(ctx context.Context, client services.ClustersClientInterface, clusterID string, nodePoolID string, maxInstances *int64, minInstances *int64) error {
	req := &cs.ModifyClusterNodePoolRequest{
		AutoScaling: &cs.ModifyClusterNodePoolRequestAutoScaling{
			Enable:       tea.Bool(true),
			MaxInstances: maxInstances,
			MinInstances: minInstances,
		},
	}
	_, err := client.ModifyClusterNodePool(ctx, &clusterID, &nodePoolID, req)
	if err != nil {
		return err
	}

	return nil
}

func UpdateNodePoolDesiredSize(ctx context.Context, client services.ClustersClientInterface, clusterID string, nodePoolID string, desiredSize *int64) error {
	req := &cs.ModifyClusterNodePoolRequest{
		ScalingGroup: &cs.ModifyClusterNodePoolRequestScalingGroup{
			DesiredSize: desiredSize,
		},
	}
	_, err := client.ModifyClusterNodePool(ctx, &clusterID, &nodePoolID, req)
	if err != nil {
		return err
	}

	return nil
}

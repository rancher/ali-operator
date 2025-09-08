package alibaba

import (
	"strconv"

	cs "github.com/alibabacloud-go/cs-20151215/v5/client"
	"github.com/alibabacloud-go/tea/tea"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	"github.com/sirupsen/logrus"
)

const (
	ClusterStatusRunning  = "running"
	ClusterStatusFailed   = "failed"
	ClusterStatusUpdating = "updating"
	ClusterStatusScaling  = "scaling"
	ClusterStatusRemoving = "removing"
)

// state of task and errors
const (
	UpdateK8sRunningStatus   = "running"
	UpdateK8sFailStatus      = "fail"
	UpdateK8sSuccessStatus   = "success"
	UpdateK8SError           = "Upgrade k8s version error"
	UpdateK8SVersionApiError = "Please check that the version of k8s to be upgraded is entered correctly"
)

// State of node pool
const (
	NodePoolStatusActive        = "active"
	NodePoolStatusInitial       = "initial"
	NodePoolStatusScaling       = "scaling"
	NodePoolStatusRemoving      = "removing"
	NodePoolStatusRemovingNodes = "removing_nodes"
	NodePoolStatusDeleting      = "deleting"
	NodePoolStatusUpdating      = "updating"
)

const (
	ManagedClusterType = "ManagedKubernetes"
)

// SyncConfigSpecClusterFieldsWithUpstream fix fields for imported clusters
func SyncConfigSpecClusterFieldsWithUpstream(configSpec *aliv1.AliClusterConfigSpec, cluster *cs.DescribeClusterDetailResponseBody) {
	// update known field from query result

	configSpec.ClusterType = tea.StringValue(cluster.ClusterType)
	if configSpec.KubernetesVersion == "" {
		configSpec.KubernetesVersion = tea.StringValue(cluster.CurrentVersion)
	}
	configSpec.ClusterSpec = tea.StringValue(cluster.ClusterSpec)
	configSpec.ClusterName = tea.StringValue(cluster.Name)
	configSpec.VSwitchIDs = tea.StringSliceValue(cluster.VswitchIds)
	configSpec.ResourceGroupID = tea.StringValue(cluster.ResourceGroupId)
	configSpec.VpcID = tea.StringValue(cluster.VpcId)
	configSpec.ProxyMode = tea.StringValue(cluster.ProxyMode)

	configSpec.ContainerCIDR = tea.StringValue(cluster.ContainerCidr)
	configSpec.ServiceCIDR = tea.StringValue(cluster.ServiceCidr)

	nodeCidrMask := tea.StringValue(cluster.NodeCidrMask)
	if nodeCidrMask != "" {
		maskNum, err := strconv.Atoi(nodeCidrMask)
		if err != nil {
			logrus.Warnf("get node-cidr-mask failed:%s", nodeCidrMask)
		} else {
			configSpec.NodeCIDRMask = maskNum
		}
	}
}

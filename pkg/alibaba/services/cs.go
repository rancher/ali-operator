package services

import (
	"context"
	"errors"

	credential "github.com/rancher/muchang/credentials"
	cs "github.com/rancher/muchang/cs/client"
	openapi "github.com/rancher/muchang/darabonba-openapi/client"
	"github.com/rancher/muchang/utils/tea"
	util "github.com/rancher/muchang/utils/tea-utils/service"
)

var errEmptyRegion = errors.New("regionId can not be empty")

type ClustersClientInterface interface {
	DescribeClusterUserKubeconfig(ctx context.Context, clusterID *string, request *cs.DescribeClusterUserKubeconfigRequest) (result *cs.DescribeClusterUserKubeconfigResponse, err error)
	DescribeClusterDetail(ctx context.Context, clusterID *string) (result *cs.DescribeClusterDetailResponse, err error)
	CreateCluster(ctx context.Context, request *cs.CreateClusterRequest) (result *cs.CreateClusterResponse, err error)
	DeleteCluster(ctx context.Context, clusterID *string, request *cs.DeleteClusterRequest) (result *cs.DeleteClusterResponse, err error)
	DescribeTaskInfo(ctx context.Context, taskID *string) (result *cs.DescribeTaskInfoResponse, err error)
	UpgradeCluster(ctx context.Context, clusterID *string, request *cs.UpgradeClusterRequest) (result *cs.UpgradeClusterResponse, err error)
	DescribeClusterNodePools(ctx context.Context, clusterID *string, request *cs.DescribeClusterNodePoolsRequest) (result *cs.DescribeClusterNodePoolsResponse, err error)
	CreateClusterNodePool(ctx context.Context, clusterID *string, request *cs.CreateClusterNodePoolRequest) (result *cs.CreateClusterNodePoolResponse, err error)
	DeleteClusterNodePool(ctx context.Context, clusterID *string, nodePoolID *string, request *cs.DeleteClusterNodepoolRequest) (result *cs.DeleteClusterNodepoolResponse, err error)
	DescribeClusterNodes(ctx context.Context, clusterID *string, request *cs.DescribeClusterNodesRequest) (result *cs.DescribeClusterNodesResponse, err error)
	RemoveNodePoolNodes(ctx context.Context, clusterID *string, nodePoolID *string, request *cs.RemoveNodePoolNodesRequest) (result *cs.RemoveNodePoolNodesResponse, err error)
	ModifyClusterNodePool(ctx context.Context, clusterID *string, nodePoolID *string, request *cs.ModifyClusterNodePoolRequest) (result *cs.ModifyClusterNodePoolResponse, err error)
}

type clustersClient struct {
	client *cs.Client
}

func NewClustersClient(creds *Credentials, regionID string) (ClustersClientInterface, error) {
	if regionID == "" {
		return nil, errEmptyRegion
	}

	credentials, err := getCredentials(creds.AccessKeyID, creds.AccessKeySecret)
	if err != nil {
		return nil, err
	}

	openAPICfg := &openapi.Config{
		Credential: credentials,
		Protocol:   tea.String("https"),
		Endpoint:   tea.String("cs." + regionID + ".aliyuncs.com"),
	}

	client, err := cs.NewClient(openAPICfg)
	if err != nil {
		return nil, err
	}

	return &clustersClient{
		client: client,
	}, nil
}

func getCredentials(ak, sk string) (credential.Credential, error) {
	config := &credential.Config{
		AccessKeyId:     &ak,
		AccessKeySecret: &sk,
		Type:            tea.String("access_key"),
	}

	return credential.NewCredential(config)
}

func (cl *clustersClient) DescribeClusterUserKubeconfig(ctx context.Context, clusterID *string, request *cs.DescribeClusterUserKubeconfigRequest) (result *cs.DescribeClusterUserKubeconfigResponse, err error) {
	return cl.client.DescribeClusterUserKubeconfigWithContext(ctx, clusterID, request, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) DescribeClusterDetail(ctx context.Context, clusterID *string) (result *cs.DescribeClusterDetailResponse, err error) {
	return cl.client.DescribeClusterDetailWithContext(ctx, clusterID, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) CreateCluster(ctx context.Context, request *cs.CreateClusterRequest) (result *cs.CreateClusterResponse, err error) {
	return cl.client.CreateClusterWithContext(ctx, request, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) DeleteCluster(ctx context.Context, clusterID *string, request *cs.DeleteClusterRequest) (result *cs.DeleteClusterResponse, err error) {
	return cl.client.DeleteClusterWithContext(ctx, clusterID, request, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) DescribeTaskInfo(ctx context.Context, taskID *string) (result *cs.DescribeTaskInfoResponse, err error) {
	return cl.client.DescribeTaskInfoWithContext(ctx, taskID, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) UpgradeCluster(ctx context.Context, clusterID *string, request *cs.UpgradeClusterRequest) (result *cs.UpgradeClusterResponse, err error) {
	return cl.client.UpgradeClusterWithContext(ctx, clusterID, request, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) DescribeClusterNodePools(ctx context.Context, clusterID *string, request *cs.DescribeClusterNodePoolsRequest) (result *cs.DescribeClusterNodePoolsResponse, err error) {
	return cl.client.DescribeClusterNodePoolsWithContext(ctx, clusterID, request, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) CreateClusterNodePool(ctx context.Context, clusterID *string, request *cs.CreateClusterNodePoolRequest) (result *cs.CreateClusterNodePoolResponse, err error) {
	return cl.client.CreateClusterNodePoolWithContext(ctx, clusterID, request, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) DescribeClusterNodes(ctx context.Context, clusterID *string, request *cs.DescribeClusterNodesRequest) (result *cs.DescribeClusterNodesResponse, err error) {
	return cl.client.DescribeClusterNodesWithContext(ctx, clusterID, request, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) DeleteClusterNodePool(ctx context.Context, clusterID *string, nodePoolID *string, request *cs.DeleteClusterNodepoolRequest) (result *cs.DeleteClusterNodepoolResponse, err error) {
	return cl.client.DeleteClusterNodepoolWithContext(ctx, clusterID, nodePoolID, request, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) RemoveNodePoolNodes(ctx context.Context, clusterID *string, nodePoolID *string, request *cs.RemoveNodePoolNodesRequest) (result *cs.RemoveNodePoolNodesResponse, err error) {
	return cl.client.RemoveNodePoolNodesWithContext(ctx, clusterID, nodePoolID, request, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) ModifyClusterNodePool(ctx context.Context, clusterID *string, nodePoolID *string, request *cs.ModifyClusterNodePoolRequest) (result *cs.ModifyClusterNodePoolResponse, err error) {
	return cl.client.ModifyClusterNodePoolWithContext(ctx, clusterID, nodePoolID, request, map[string]*string{}, &util.RuntimeOptions{})
}

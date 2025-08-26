package services

import (
	"context"
	"errors"

	cs "github.com/alibabacloud-go/cs-20151215/v5/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	credential "github.com/aliyun/credentials-go/credentials"
)

var errEmptyRegion = errors.New("regionId can not be empty")

type ClustersClientInterface interface {
	DescribeClusterUserKubeconfig(ctx context.Context, clusterId *string, request *cs.DescribeClusterUserKubeconfigRequest) (result *cs.DescribeClusterUserKubeconfigResponse, err error)
	DescribeClusterDetail(ctx context.Context, clusterId *string) (result *cs.DescribeClusterDetailResponse, err error)
	CreateCluster(ctx context.Context, request *cs.CreateClusterRequest) (result *cs.CreateClusterResponse, err error)
	DeleteCluster(ctx context.Context, clusterID *string, request *cs.DeleteClusterRequest) (result *cs.DeleteClusterResponse, err error)
}

type clustersClient struct {
	client *cs.Client
}

func NewClustersClient(creds *Credentials, regionId string) (*clustersClient, error) {
	if regionId == "" {
		return nil, errEmptyRegion
	}

	credentials, err := getCredentials(creds.AccessKeyID, creds.AccessKeySecret)
	if err != nil {
		return nil, err
	}

	openAPICfg := &openapi.Config{
		Credential: credentials,
		Protocol:   tea.String("https"),
		Endpoint:   tea.String("cs." + regionId + ".aliyuncs.com"),
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

func (cl *clustersClient) DescribeClusterUserKubeconfig(ctx context.Context, clusterId *string, request *cs.DescribeClusterUserKubeconfigRequest) (result *cs.DescribeClusterUserKubeconfigResponse, err error) {
	return cl.client.DescribeClusterUserKubeconfigWithOptions(clusterId, request, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) DescribeClusterDetail(ctx context.Context, clusterId *string) (result *cs.DescribeClusterDetailResponse, err error) {
	return cl.client.DescribeClusterDetailWithOptions(clusterId, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) CreateCluster(ctx context.Context, request *cs.CreateClusterRequest) (result *cs.CreateClusterResponse, err error) {
	return cl.client.CreateClusterWithOptions(request, map[string]*string{}, &util.RuntimeOptions{})
}

func (cl *clustersClient) DeleteCluster(ctx context.Context, clusterID *string, request *cs.DeleteClusterRequest) (result *cs.DeleteClusterResponse, err error) {
	return cl.client.DeleteClusterWithOptions(clusterID, request, map[string]*string{}, &util.RuntimeOptions{})
}

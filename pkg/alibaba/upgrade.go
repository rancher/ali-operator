package alibaba

import (
	"context"
	"errors"

	cs "github.com/rancher/muchang/cs/client"

	"github.com/rancher/ali-operator/pkg/alibaba/services"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
)

func UpgradeCluster(ctx context.Context, client services.ClustersClientInterface, configSpec *aliv1.AliClusterConfigSpec) (string, error) {
	request := cs.UpgradeClusterRequest{
		NextVersion: &configSpec.KubernetesVersion,
	}

	upgradeResp, err := client.UpgradeCluster(ctx, &configSpec.ClusterID, &request)
	if err != nil {
		return "", err
	}
	if upgradeResp == nil || upgradeResp.Body == nil {
		return "", errors.New("received empty upgrade cluster response")
	}

	return *upgradeResp.Body.TaskId, nil
}

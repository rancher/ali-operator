package alibaba

import (
	"context"

	"github.com/rancher/ali-operator/pkg/alibaba/services"
	cs "github.com/rancher/muchang/cs/client"
)

func GetTask(ctx context.Context, clustersClient services.ClustersClientInterface, taskID string) (*cs.DescribeTaskInfoResponseBody, error) {
	taskResp, err := clustersClient.DescribeTaskInfo(ctx, &taskID)
	if err != nil {
		return nil, err
	}
	if taskResp == nil || taskResp.Body == nil {
		return nil, nil
	}
	return taskResp.Body, nil
}

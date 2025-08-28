package alibaba

import (
	"context"

	cs "github.com/alibabacloud-go/cs-20151215/v5/client"
	"github.com/rancher/ali-operator/pkg/alibaba/services"
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

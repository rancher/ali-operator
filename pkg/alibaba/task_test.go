package alibaba

import (
	"context"
	"errors"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/ali-operator/pkg/alibaba/services/mock_services"
	cs "github.com/rancher/muchang/cs/client"
	"github.com/rancher/muchang/utils/tea"
)

var _ = Describe("GetTask", func() {
	var (
		clustersClientMock *mock_services.MockClustersClientInterface
		ctrl               *gomock.Controller
		taskID             string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)
		taskID = "test-task-id"
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should get task successfully", func() {
		expectedTask := &cs.DescribeTaskInfoResponseBody{
			State:  tea.String("success"),
			TaskId: tea.String(taskID),
		}
		resp := &cs.DescribeTaskInfoResponse{
			Body: expectedTask,
		}
		clustersClientMock.EXPECT().DescribeTaskInfo(gomock.Any(), &taskID).Return(resp, nil)

		task, err := GetTask(context.Background(), clustersClientMock, taskID)
		Expect(err).ToNot(HaveOccurred())
		Expect(task).To(Equal(expectedTask))
	})

	It("should fail if DescribeTaskInfo API returns an error", func() {
		apiError := errors.New("API error")
		clustersClientMock.EXPECT().DescribeTaskInfo(gomock.Any(), &taskID).Return(nil, apiError)

		_, err := GetTask(context.Background(), clustersClientMock, taskID)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(apiError))
	})

	It("should return nil if DescribeTaskInfo API returns a nil response", func() {
		clustersClientMock.EXPECT().DescribeTaskInfo(gomock.Any(), &taskID).Return(nil, nil)

		task, err := GetTask(context.Background(), clustersClientMock, taskID)
		Expect(err).ToNot(HaveOccurred())
		Expect(task).To(BeNil())
	})

	It("should return nil if DescribeTaskInfo API returns a response with a nil body", func() {
		resp := &cs.DescribeTaskInfoResponse{
			Body: nil,
		}
		clustersClientMock.EXPECT().DescribeTaskInfo(gomock.Any(), &taskID).Return(resp, nil)

		task, err := GetTask(context.Background(), clustersClientMock, taskID)
		Expect(err).ToNot(HaveOccurred())
		Expect(task).To(BeNil())
	})
})

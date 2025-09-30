package alibaba

import (
	"context"
	"errors"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/ali-operator/pkg/alibaba/services/mock_services"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	cs "github.com/rancher/muchang/cs/client"
	"github.com/rancher/muchang/utils/tea"
)

var _ = Describe("UpgradeCluster", func() {
	var (
		clustersClientMock *mock_services.MockClustersClientInterface
		ctrl               *gomock.Controller
		configSpec         *aliv1.AliClusterConfigSpec
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)
		configSpec = &aliv1.AliClusterConfigSpec{
			ClusterID:         "test-cluster-id",
			KubernetesVersion: "1.32.7-aliyun.1",
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should upgrade cluster successfully", func() {
		expectedTaskID := "t-12345"
		resp := &cs.UpgradeClusterResponse{
			Body: &cs.UpgradeClusterResponseBody{
				TaskId: tea.String(expectedTaskID),
			},
		}
		clustersClientMock.EXPECT().UpgradeCluster(gomock.Any(), &configSpec.ClusterID, gomock.Any()).Return(resp, nil)

		taskID, err := UpgradeCluster(context.Background(), clustersClientMock, configSpec)
		Expect(err).ToNot(HaveOccurred())
		Expect(taskID).To(Equal(expectedTaskID))
	})

	It("should fail if UpgradeCluster API returns an error", func() {
		apiError := errors.New("API error")
		clustersClientMock.EXPECT().UpgradeCluster(gomock.Any(), &configSpec.ClusterID, gomock.Any()).Return(nil, apiError)

		_, err := UpgradeCluster(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(apiError))
	})

	It("should fail if UpgradeCluster API returns a nil response", func() {
		clustersClientMock.EXPECT().UpgradeCluster(gomock.Any(), &configSpec.ClusterID, gomock.Any()).Return(nil, nil)

		_, err := UpgradeCluster(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("received empty upgrade cluster response"))
	})

	It("should fail if UpgradeCluster API returns a response with a nil body", func() {
		resp := &cs.UpgradeClusterResponse{
			Body: nil,
		}
		clustersClientMock.EXPECT().UpgradeCluster(gomock.Any(), &configSpec.ClusterID, gomock.Any()).Return(resp, nil)

		_, err := UpgradeCluster(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("received empty upgrade cluster response"))
	})
})

package alibaba

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/ali-operator/pkg/alibaba/services/mock_services"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	cs "github.com/rancher/muchang/cs/client"
	"github.com/rancher/muchang/utils/tea"
)

var _ = Describe("Create", func() {
	var (
		clustersClientMock *mock_services.MockClustersClientInterface
		ctrl               *gomock.Controller
		configSpec         *aliv1.AliClusterConfigSpec
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)
		configSpec = &aliv1.AliClusterConfigSpec{
			ClusterName: "test-cluster",
			RegionID:    "eu-central",
			ClusterType: ManagedClusterType,
			VpcID:       "vpc-id",
			VSwitchIDs:  []string{"vswitch-id"},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should create cluster successfully", func() {
		expectedClusterID := "c12345"
		resp := &cs.CreateClusterResponse{
			Body: &cs.CreateClusterResponseBody{
				ClusterId: tea.String(expectedClusterID),
			},
		}
		clustersClientMock.EXPECT().CreateCluster(gomock.Any(), gomock.Any()).Return(resp, nil)

		clusterID, err := Create(context.Background(), clustersClientMock, configSpec)
		Expect(err).ToNot(HaveOccurred())
		Expect(clusterID).To(Equal(expectedClusterID))
	})

	It("should fail if cluster name is not provided", func() {
		configSpec.ClusterName = ""
		_, err := Create(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(ErrRequiredClusterName))
	})

	It("should fail if region id is not provided", func() {
		configSpec.RegionID = ""
		_, err := Create(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(ErrRequiredRegionID))
	})

	It("should fail if cluster type is invalid", func() {
		configSpec.ClusterType = "invalid-type"
		_, err := Create(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf(ErrInvalidClusterType, configSpec.ClusterType)))
	})

	It("should fail if CreateCluster API returns an error", func() {
		apiError := errors.New("API error")
		clustersClientMock.EXPECT().CreateCluster(gomock.Any(), gomock.Any()).Return(nil, apiError)

		_, err := Create(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(apiError))
	})

	It("should fail if CreateCluster API returns a nil response", func() {
		clustersClientMock.EXPECT().CreateCluster(gomock.Any(), gomock.Any()).Return(nil, nil)

		_, err := Create(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("received invalid cluster response"))
	})

	It("should fail if CreateCluster API returns a response with a nil body", func() {
		resp := &cs.CreateClusterResponse{
			Body: nil,
		}
		clustersClientMock.EXPECT().CreateCluster(gomock.Any(), gomock.Any()).Return(resp, nil)

		_, err := Create(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("received invalid cluster response"))
	})

	It("should fail if vpcId is not provided when zoneIds are empty", func() {
		configSpec.VpcID = ""
		configSpec.ZoneIDs = nil
		_, err := Create(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("vpcId is required if zoneIds are not provided"))
	})

	It("should fail if vSwitchIds are not provided when zoneIds are empty", func() {
		configSpec.VSwitchIDs = nil
		configSpec.ZoneIDs = nil
		_, err := Create(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("vSwitchIds are required if zoneIds are not provided"))
	})

	It("should fail if zoneIds are provided along with vpcId", func() {
		configSpec.ZoneIDs = []string{"zone-a"}
		configSpec.VpcID = "vpc-id"
		_, err := Create(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("zoneIds should not be used together with vpcId and vSwitchIds"))
	})

	It("should fail if zoneIds are provided along with vSwitchIds", func() {
		configSpec.ZoneIDs = []string{"zone-a"}
		configSpec.VpcID = ""
		configSpec.VSwitchIDs = []string{"vswitch-id"}
		_, err := Create(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("zoneIds should not be used together with vpcId and vSwitchIds"))
	})
})

var _ = Describe("GetCluster", func() {
	var (
		clustersClientMock *mock_services.MockClustersClientInterface
		ctrl               *gomock.Controller
		clusterID          string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)
		clusterID = "test-cluster-id"
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should get cluster successfully", func() {
		expectedCluster := &cs.DescribeClusterDetailResponseBody{
			Name:      tea.String("my-cluster"),
			ClusterId: tea.String(clusterID),
		}
		resp := &cs.DescribeClusterDetailResponse{
			Body: expectedCluster,
		}
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &clusterID).Return(resp, nil)

		cluster, err := GetCluster(context.Background(), clustersClientMock, clusterID)
		Expect(err).ToNot(HaveOccurred())
		Expect(cluster).To(Equal(expectedCluster))
	})

	It("should fail if DescribeClusterDetail API returns an error", func() {
		apiError := errors.New("API error")
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &clusterID).Return(nil, apiError)

		_, err := GetCluster(context.Background(), clustersClientMock, clusterID)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(apiError))
	})

	It("should fail if DescribeClusterDetail API returns a nil response", func() {
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &clusterID).Return(nil, nil)

		_, err := GetCluster(context.Background(), clustersClientMock, clusterID)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("the cluster is nil, indicating no cluster information is available"))
	})

	It("should fail if DescribeClusterDetail API returns a response with a nil body", func() {
		resp := &cs.DescribeClusterDetailResponse{
			Body: nil,
		}
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &clusterID).Return(resp, nil)

		_, err := GetCluster(context.Background(), clustersClientMock, clusterID)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("the cluster is nil, indicating no cluster information is available"))
	})
})

var _ = Describe("GetAddons", func() {
	It("should return nil if config spec is nil", func() {
		addons := GetAddons(nil)
		Expect(addons).To(BeNil())
	})

	It("should return nil if there are no addons in config spec", func() {
		configSpec := &aliv1.AliClusterConfigSpec{
			Addons: []aliv1.AliAddon{},
		}
		addons := GetAddons(configSpec)
		Expect(addons).To(BeNil())
	})

	It("should convert addons successfully", func() {
		configSpec := &aliv1.AliClusterConfigSpec{
			Addons: []aliv1.AliAddon{
				{
					Name:   "test-cni-eniip",
					Config: `{"VSwitchID": "vsw-123"}`,
				},
				{
					Name:   "test-2",
					Config: `{}`,
				},
			},
		}

		addons := GetAddons(configSpec)
		Expect(addons).ToNot(BeNil())
		Expect(len(addons)).To(Equal(2))
		Expect(*addons[0].Name).To(Equal("test-cni-eniip"))
		Expect(*addons[0].Config).To(Equal(`{"VSwitchID": "vsw-123"}`))
		Expect(*addons[1].Name).To(Equal("test-2"))
		Expect(*addons[1].Config).To(Equal(`{}`))
	})
})

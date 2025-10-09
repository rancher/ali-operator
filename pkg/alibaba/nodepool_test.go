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

var _ = Describe("GetNodePools", func() {
	var (
		clustersClientMock *mock_services.MockClustersClientInterface
		ctrl               *gomock.Controller
		configSpec         *aliv1.AliClusterConfigSpec
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)
		configSpec = &aliv1.AliClusterConfigSpec{
			ClusterID: "test-cluster-id",
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should get nodepools successfully", func() {
		expectedNodepools := []*cs.DescribeClusterNodePoolsResponseBodyNodepools{
			{
				NodepoolInfo: &cs.DescribeClusterNodePoolsResponseBodyNodepoolsNodepoolInfo{
					Name: tea.String("test-nodepool"),
				},
			},
		}
		resp := &cs.DescribeClusterNodePoolsResponse{
			Body: &cs.DescribeClusterNodePoolsResponseBody{
				Nodepools: expectedNodepools,
			},
		}
		clustersClientMock.EXPECT().DescribeClusterNodePools(gomock.Any(), &configSpec.ClusterID, gomock.Any()).Return(resp, nil)

		nodepools, err := GetNodePools(context.Background(), clustersClientMock, configSpec)
		Expect(err).ToNot(HaveOccurred())
		Expect(nodepools).To(Equal(expectedNodepools))
	})

	It("should fail if API returns an error", func() {
		apiError := errors.New("API error")
		clustersClientMock.EXPECT().DescribeClusterNodePools(gomock.Any(), &configSpec.ClusterID, gomock.Any()).Return(nil, apiError)

		_, err := GetNodePools(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(apiError))
	})

	It("should fail if API returns a nil response", func() {
		clustersClientMock.EXPECT().DescribeClusterNodePools(gomock.Any(), &configSpec.ClusterID, gomock.Any()).Return(nil, nil)

		_, err := GetNodePools(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(ErrEmptyClusterNodePools))
	})

	It("should fail if API returns a response with a nil body", func() {
		resp := &cs.DescribeClusterNodePoolsResponse{Body: nil}
		clustersClientMock.EXPECT().DescribeClusterNodePools(gomock.Any(), &configSpec.ClusterID, gomock.Any()).Return(resp, nil)

		_, err := GetNodePools(context.Background(), clustersClientMock, configSpec)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(ErrEmptyClusterNodePools))
	})
})

var _ = Describe("ToNodePoolConfig", func() {
	It("should convert nodepools successfully", func() {
		upstreamNodePools := []*cs.DescribeClusterNodePoolsResponseBodyNodepools{
			{
				NodepoolInfo: &cs.DescribeClusterNodePoolsResponseBodyNodepoolsNodepoolInfo{
					NodepoolId: tea.String("np-1"),
					Name:       tea.String("pool-1"),
				},
				AutoScaling: &cs.DescribeClusterNodePoolsResponseBodyNodepoolsAutoScaling{
					Enable:       tea.Bool(true),
					MaxInstances: tea.Int64(5),
					MinInstances: tea.Int64(1),
				},
				ScalingGroup: &cs.DescribeClusterNodePoolsResponseBodyNodepoolsScalingGroup{
					DesiredSize: tea.Int64(3),
				},
				KubernetesConfig: &cs.DescribeClusterNodePoolsResponseBodyNodepoolsKubernetesConfig{
					Runtime:        tea.String("testruntime"),
					RuntimeVersion: tea.String("1.0"),
				},
			},
		}

		config := ToNodePoolConfig(upstreamNodePools)
		Expect(config).To(HaveLen(1))
		Expect(config[0].NodePoolID).To(Equal("np-1"))
		Expect(config[0].Name).To(Equal("pool-1"))
		Expect(*config[0].EnableAutoScaling).To(BeTrue())
		Expect(*config[0].MaxInstances).To(Equal(int64(5)))
		Expect(*config[0].MinInstances).To(Equal(int64(1)))
		Expect(*config[0].DesiredSize).To(Equal(int64(3)))
		Expect(config[0].Runtime).To(Equal("testruntime"))
		Expect(config[0].RuntimeVersion).To(Equal("1.0"))
	})

	It("should handle empty input", func() {
		config := ToNodePoolConfig([]*cs.DescribeClusterNodePoolsResponseBodyNodepools{})
		Expect(config).To(BeEmpty())
	})

	It("should handle nil fields gracefully", func() {
		upstreamNodePools := []*cs.DescribeClusterNodePoolsResponseBodyNodepools{
			{
				NodepoolInfo: &cs.DescribeClusterNodePoolsResponseBodyNodepoolsNodepoolInfo{
					NodepoolId: tea.String("np-1"),
				},
				AutoScaling:  &cs.DescribeClusterNodePoolsResponseBodyNodepoolsAutoScaling{},
				ScalingGroup: &cs.DescribeClusterNodePoolsResponseBodyNodepoolsScalingGroup{},
			},
		}
		config := ToNodePoolConfig(upstreamNodePools)
		Expect(config).To(HaveLen(1))
		Expect(config[0].NodePoolID).To(Equal("np-1"))
		Expect(config[0].EnableAutoScaling).To(BeNil())
		Expect(config[0].DesiredSize).To(BeNil())
	})
})

var _ = Describe("CreateNodePool", func() {
	var (
		clustersClientMock *mock_services.MockClustersClientInterface
		ctrl               *gomock.Controller
		configSpec         *aliv1.AliClusterConfigSpec
		npConfig           *aliv1.AliNodePool
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)
		configSpec = &aliv1.AliClusterConfigSpec{
			ClusterID:  "test-cluster-id",
			VSwitchIDs: []string{"cluster-vswitch"},
		}
		npConfig = &aliv1.AliNodePool{
			Name: "test-nodepool",
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should create nodepool successfully", func() {
		expectedRespBody := &cs.CreateClusterNodePoolResponseBody{
			NodepoolId: tea.String("np-123"),
		}
		resp := &cs.CreateClusterNodePoolResponse{Body: expectedRespBody}
		clustersClientMock.EXPECT().CreateClusterNodePool(gomock.Any(), &configSpec.ClusterID, gomock.Any()).Return(resp, nil)

		result, err := CreateNodePool(context.Background(), clustersClientMock, configSpec, npConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(expectedRespBody))
	})

	It("should use cluster vSwitch IDs if nodepool vSwitch IDs are not provided", func() {
		npConfig.VSwitchIDs = nil
		matcher := gomock.AssignableToTypeOf(&cs.CreateClusterNodePoolRequest{})
		clustersClientMock.EXPECT().CreateClusterNodePool(gomock.Any(), &configSpec.ClusterID, matcher).
			DoAndReturn(func(_ context.Context, _ *string, req *cs.CreateClusterNodePoolRequest) (*cs.CreateClusterNodePoolResponse, error) {
				Expect(tea.StringSliceValue(req.ScalingGroup.VswitchIds)).To(Equal(configSpec.VSwitchIDs))
				return &cs.CreateClusterNodePoolResponse{Body: &cs.CreateClusterNodePoolResponseBody{}}, nil
			})

		_, err := CreateNodePool(context.Background(), clustersClientMock, configSpec, npConfig)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should use nodepool vSwitch IDs if provided", func() {
		npConfig.VSwitchIDs = []string{"nodepool-vswitch"}
		matcher := gomock.AssignableToTypeOf(&cs.CreateClusterNodePoolRequest{})
		clustersClientMock.EXPECT().CreateClusterNodePool(gomock.Any(), &configSpec.ClusterID, matcher).
			DoAndReturn(func(_ context.Context, _ *string, req *cs.CreateClusterNodePoolRequest) (*cs.CreateClusterNodePoolResponse, error) {
				Expect(tea.StringSliceValue(req.ScalingGroup.VswitchIds)).To(Equal(npConfig.VSwitchIDs))
				return &cs.CreateClusterNodePoolResponse{Body: &cs.CreateClusterNodePoolResponseBody{}}, nil
			})

		_, err := CreateNodePool(context.Background(), clustersClientMock, configSpec, npConfig)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should fail if API returns an error", func() {
		apiError := errors.New("API error")
		clustersClientMock.EXPECT().CreateClusterNodePool(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, apiError)

		_, err := CreateNodePool(context.Background(), clustersClientMock, configSpec, npConfig)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(apiError))
	})
})

var _ = Describe("DeleteNodePool", func() {
	var (
		clustersClientMock *mock_services.MockClustersClientInterface
		ctrl               *gomock.Controller
		clusterID          string
		nodePool           *aliv1.AliNodePool
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)
		clusterID = "test-cluster-id"
		nodePool = &aliv1.AliNodePool{
			NodePoolID: "np-123",
			Name:       "test-nodepool",
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should scale down desired size to 0 before deleting", func() {
		nodePool.DesiredSize = tea.Int64(1)
		clustersClientMock.EXPECT().ModifyClusterNodePool(gomock.Any(), &clusterID, &nodePool.NodePoolID, gomock.Any()).Return(nil, nil)

		err := DeleteNodePool(context.Background(), clustersClientMock, clusterID, nodePool)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should scale down autoscaling to 0 before deleting", func() {
		nodePool.MinInstances = tea.Int64(1)
		nodePool.MaxInstances = tea.Int64(2)
		clustersClientMock.EXPECT().ModifyClusterNodePool(gomock.Any(), &clusterID, &nodePool.NodePoolID, gomock.Any()).Return(nil, nil)

		err := DeleteNodePool(context.Background(), clustersClientMock, clusterID, nodePool)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should remove existing nodes before deleting", func() {
		nodesResp := &cs.DescribeClusterNodesResponse{
			Body: &cs.DescribeClusterNodesResponseBody{
				Nodes: []*cs.DescribeClusterNodesResponseBodyNodes{
					{InstanceId: tea.String("i-123")},
				},
			},
		}
		clustersClientMock.EXPECT().DescribeClusterNodes(gomock.Any(), &clusterID, gomock.Any()).Return(nodesResp, nil)
		clustersClientMock.EXPECT().RemoveNodePoolNodes(gomock.Any(), &clusterID, &nodePool.NodePoolID, gomock.Any()).Return(nil, nil)

		err := DeleteNodePool(context.Background(), clustersClientMock, clusterID, nodePool)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should delete the nodepool directly if it has no nodes and is scaled to 0", func() {
		nodesResp := &cs.DescribeClusterNodesResponse{
			Body: &cs.DescribeClusterNodesResponseBody{
				Nodes: []*cs.DescribeClusterNodesResponseBodyNodes{},
			},
		}
		clustersClientMock.EXPECT().DescribeClusterNodes(gomock.Any(), &clusterID, gomock.Any()).Return(nodesResp, nil)
		clustersClientMock.EXPECT().DeleteClusterNodePool(gomock.Any(), &clusterID, &nodePool.NodePoolID, gomock.Any()).Return(nil, nil)

		err := DeleteNodePool(context.Background(), clustersClientMock, clusterID, nodePool)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should return an error if deleting nodepool fails", func() {
		apiError := errors.New("API error")
		nodesResp := &cs.DescribeClusterNodesResponse{
			Body: &cs.DescribeClusterNodesResponseBody{
				Nodes: []*cs.DescribeClusterNodesResponseBodyNodes{},
			},
		}
		clustersClientMock.EXPECT().DescribeClusterNodes(gomock.Any(), &clusterID, gomock.Any()).Return(nodesResp, nil)
		clustersClientMock.EXPECT().DeleteClusterNodePool(gomock.Any(), &clusterID, &nodePool.NodePoolID, gomock.Any()).Return(nil, apiError)

		err := DeleteNodePool(context.Background(), clustersClientMock, clusterID, nodePool)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf("failed to delete nodepool %s: %s", nodePool.NodePoolID, apiError.Error())))
	})
})

var _ = Describe("WaitForNodePool", func() {
	It("should return true for scaling states", func() {
		Expect(WaitForNodePool(tea.String(NodePoolStatusScaling))).To(BeTrue())
		Expect(WaitForNodePool(tea.String(NodePoolStatusDeleting))).To(BeTrue())
		Expect(WaitForNodePool(tea.String(NodePoolStatusInitial))).To(BeTrue())
		Expect(WaitForNodePool(tea.String(NodePoolStatusUpdating))).To(BeTrue())
		Expect(WaitForNodePool(tea.String(NodePoolStatusRemoving))).To(BeTrue())
		Expect(WaitForNodePool(tea.String(NodePoolStatusRemovingNodes))).To(BeTrue())
	})

	It("should return false for stable states", func() {
		Expect(WaitForNodePool(tea.String(NodePoolStatusActive))).To(BeFalse())
	})

	It("should return false for nil state", func() {
		Expect(WaitForNodePool(nil)).To(BeFalse())
	})

	It("should return false for unknown state", func() {
		Expect(WaitForNodePool(tea.String("unknown-state"))).To(BeFalse())
	})
})

var _ = Describe("UpdateNodePoolAutoScalingConfig", func() {
	var (
		clustersClientMock *mock_services.MockClustersClientInterface
		ctrl               *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should update autoscaling config successfully", func() {
		clustersClientMock.EXPECT().ModifyClusterNodePool(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		err := UpdateNodePoolAutoScalingConfig(context.Background(), clustersClientMock, "c1", "np1", tea.Int64(5), tea.Int64(1))
		Expect(err).ToNot(HaveOccurred())
	})

	It("should return an error if API fails", func() {
		apiError := errors.New("API error")
		clustersClientMock.EXPECT().ModifyClusterNodePool(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, apiError)
		err := UpdateNodePoolAutoScalingConfig(context.Background(), clustersClientMock, "c1", "np1", tea.Int64(5), tea.Int64(1))
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(apiError))
	})
})

var _ = Describe("UpdateNodePoolDesiredSize", func() {
	var (
		clustersClientMock *mock_services.MockClustersClientInterface
		ctrl               *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should update desired size successfully", func() {
		clustersClientMock.EXPECT().ModifyClusterNodePool(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		err := UpdateNodePoolDesiredSize(context.Background(), clustersClientMock, "c1", "np1", tea.Int64(3))
		Expect(err).ToNot(HaveOccurred())
	})

	It("should return an error if API fails", func() {
		apiError := errors.New("API error")
		clustersClientMock.EXPECT().ModifyClusterNodePool(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, apiError)
		err := UpdateNodePoolDesiredSize(context.Background(), clustersClientMock, "c1", "np1", tea.Int64(3))
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(apiError))
	})
})

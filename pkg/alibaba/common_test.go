package alibaba

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	cs "github.com/rancher/muchang/cs/client"
	"github.com/rancher/muchang/utils/tea"
)

var _ = Describe("SyncConfigSpecClusterFieldsWithUpstream", func() {
	var (
		configSpec *aliv1.AliClusterConfigSpec
		cluster    *cs.DescribeClusterDetailResponseBody
	)

	BeforeEach(func() {
		configSpec = &aliv1.AliClusterConfigSpec{}
		cluster = &cs.DescribeClusterDetailResponseBody{
			ClusterType:     tea.String("ManagedKubernetes"),
			CurrentVersion:  tea.String("1.32.7-aliyun.1"),
			ClusterSpec:     tea.String("ack.pro.small"),
			Name:            tea.String("upstream-cluster-name"),
			VswitchIds:      tea.StringSlice([]string{"vsw-123", "vsw-456"}),
			ResourceGroupId: tea.String("rg-abcdef"),
			VpcId:           tea.String("vpc-12345"),
			ProxyMode:       tea.String("ipvs"),
			ContainerCidr:   tea.String("172.16.0.0/16"),
			ServiceCidr:     tea.String("172.17.0.0/16"),
			NodeCidrMask:    tea.String("24"),
		}
	})

	It("should sync all fields from upstream to an empty spec", func() {
		SyncConfigSpecClusterFieldsWithUpstream(configSpec, cluster)

		Expect(configSpec.ClusterType).To(Equal("ManagedKubernetes"))
		Expect(configSpec.KubernetesVersion).To(Equal("1.32.7-aliyun.1"))
		Expect(configSpec.ClusterSpec).To(Equal("ack.pro.small"))
		Expect(configSpec.ClusterName).To(Equal("upstream-cluster-name"))
		Expect(configSpec.VSwitchIDs).To(Equal([]string{"vsw-123", "vsw-456"}))
		Expect(configSpec.ResourceGroupID).To(Equal("rg-abcdef"))
		Expect(configSpec.VpcID).To(Equal("vpc-12345"))
		Expect(configSpec.ProxyMode).To(Equal("ipvs"))
		Expect(configSpec.ContainerCIDR).To(Equal("172.16.0.0/16"))
		Expect(configSpec.ServiceCIDR).To(Equal("172.17.0.0/16"))
		Expect(configSpec.NodeCIDRMask).To(Equal(24))
	})

	It("should not overwrite an existing KubernetesVersion in the spec", func() {
		configSpec.KubernetesVersion = "1.32.7-aliyun.1"
		SyncConfigSpecClusterFieldsWithUpstream(configSpec, cluster)
		Expect(configSpec.KubernetesVersion).To(Equal("1.32.7-aliyun.1"))
	})

	It("should handle nil or empty fields from upstream gracefully", func() {
		cluster.ProxyMode = nil
		cluster.NodeCidrMask = tea.String("")
		SyncConfigSpecClusterFieldsWithUpstream(configSpec, cluster)
		Expect(configSpec.ProxyMode).To(BeEmpty())
		Expect(configSpec.NodeCIDRMask).To(BeZero())
	})
})

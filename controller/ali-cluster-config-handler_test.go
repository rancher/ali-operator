package controller

import (
	"context"
	"errors"

	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/ali-operator/pkg/alibaba"
	"github.com/rancher/ali-operator/pkg/alibaba/services/mock_services"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	"github.com/rancher/ali-operator/pkg/test"
	cs "github.com/rancher/muchang/cs/client"
	"github.com/rancher/muchang/utils/tea"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("importCluster", func() {
	var (
		handler            *Handler
		ctrl               *gomock.Controller
		clustersClientMock *mock_services.MockClustersClientInterface
		config             *aliv1.AliClusterConfig
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)

		handler = &Handler{
			aliCC:        aliFactory.Ali().V1().AliClusterConfig(),
			secrets:      coreFactory.Core().V1().Secret(),
			secretsCache: coreFactory.Core().V1().Secret().Cache(),
			alibabaClients: alibabaClients{
				clustersClient: clustersClientMock,
			},
		}

		config = &aliv1.AliClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
				UID:       "test-uid",
			},
			Spec: aliv1.AliClusterConfigSpec{
				ClusterID: "c12345",
			},
			Status: aliv1.AliClusterConfigStatus{
				Phase: aliConfigImportingPhase,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(test.CleanupAndWait(ctx, cl, config)).To(Succeed())
	})

	It("should import cluster successfully", func() {
		clusterDetailResp := &cs.DescribeClusterDetailResponse{
			Body: &cs.DescribeClusterDetailResponseBody{State: tea.String(alibaba.ClusterStatusRunning)},
		}
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &config.Spec.ClusterID).Return(clusterDetailResp, nil)

		kubeconfigResp := &cs.DescribeClusterUserKubeconfigResponse{
			Body: &cs.DescribeClusterUserKubeconfigResponseBody{
				Config: tea.String(`apiVersion: v1
clusters:
- cluster: {server: 'https://1.2.3.4:6443', certificate-authority-data: 'Cg=='}
  name: kubernetes
contexts:
- context: {cluster: kubernetes, user: ""}
  name: kubernetes
current-context: kubernetes
kind: Config
users: []`),
			},
		}
		clustersClientMock.EXPECT().DescribeClusterUserKubeconfig(gomock.Any(), &config.Spec.ClusterID, gomock.Any()).Return(kubeconfigResp, nil)

		createdConfig, err := aliFactory.Ali().V1().AliClusterConfig().Create(config)
		Expect(err).NotTo(HaveOccurred())

		updatedConfig, err := handler.importCluster(createdConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedConfig.Status.Phase).To(Equal(aliConfigActivePhase))

		secret, err := coreFactory.Core().V1().Secret().Get(config.Namespace, config.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Data["endpoint"]).To(Equal([]byte("https://1.2.3.4:6443")))
		Expect(secret.Data["ca"]).ToNot(BeEmpty())
	})

	It("should fail if DescribeClusterDetail returns an error", func() {
		apiError := errors.New("api error")
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &config.Spec.ClusterID).Return(nil, apiError)

		_, err := handler.importCluster(config)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(apiError))
	})

	It("should fail if cluster is not in running state", func() {
		clusterDetailResp := &cs.DescribeClusterDetailResponse{
			Body: &cs.DescribeClusterDetailResponseBody{State: tea.String("updating")},
		}
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &config.Spec.ClusterID).Return(clusterDetailResp, nil)

		_, err := handler.importCluster(config)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf("the current cluster status is %s. Please wait for the cluster to become %s", "updating", alibaba.ClusterStatusRunning)))
	})

	It("should fail if createCASecret fails", func() {
		clusterDetailResp := &cs.DescribeClusterDetailResponse{
			Body: &cs.DescribeClusterDetailResponseBody{State: tea.String(alibaba.ClusterStatusRunning)},
		}
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &config.Spec.ClusterID).Return(clusterDetailResp, nil)

		apiError := errors.New("kubeconfig error")
		clustersClientMock.EXPECT().DescribeClusterUserKubeconfig(gomock.Any(), &config.Spec.ClusterID, gomock.Any()).Return(nil, apiError)

		_, err := handler.importCluster(config)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(apiError))
	})

	It("should fail if UpdateStatus fails", func() {
		clusterDetailResp := &cs.DescribeClusterDetailResponse{
			Body: &cs.DescribeClusterDetailResponseBody{State: tea.String(alibaba.ClusterStatusRunning)},
		}
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &config.Spec.ClusterID).Return(clusterDetailResp, nil)

		kubeconfigResp := &cs.DescribeClusterUserKubeconfigResponse{
			Body: &cs.DescribeClusterUserKubeconfigResponseBody{
				Config: tea.String(`apiVersion: v1
clusters:
- cluster: {server: 'https://1.2.3.4:6443', certificate-authority-data: 'Cg=='}
  name: kubernetes
contexts:
- context: {cluster: kubernetes, user: ""}
  name: kubernetes
current-context: kubernetes
kind: Config
users: []`),
			},
		}
		clustersClientMock.EXPECT().DescribeClusterUserKubeconfig(gomock.Any(), &config.Spec.ClusterID, gomock.Any()).Return(kubeconfigResp, nil)

		// Not creating the config object causes UpdateStatus to fail
		_, err := handler.importCluster(config)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("needsUpdate", func() {
	It("should return false if desired and upstream are identical", func() {
		desired := []aliv1.AliNodePool{
			{NodePoolID: "np1", MaxInstances: tea.Int64(5), MinInstances: tea.Int64(1), EnableAutoScaling: tea.Bool(true), DesiredSize: tea.Int64(3)},
		}
		upstream := map[string]aliv1.AliNodePool{
			"np1": {NodePoolID: "np1", MaxInstances: tea.Int64(5), MinInstances: tea.Int64(1), EnableAutoScaling: tea.Bool(true), DesiredSize: tea.Int64(3)},
		}
		Expect(needsUpdate(desired, upstream)).To(BeFalse())
	})

	It("should return true if MaxInstances differs", func() {
		desired := []aliv1.AliNodePool{
			{NodePoolID: "np1", MaxInstances: tea.Int64(6), MinInstances: tea.Int64(1), EnableAutoScaling: tea.Bool(true)},
		}
		upstream := map[string]aliv1.AliNodePool{
			"np1": {NodePoolID: "np1", MaxInstances: tea.Int64(5), MinInstances: tea.Int64(1), EnableAutoScaling: tea.Bool(true)},
		}
		Expect(needsUpdate(desired, upstream)).To(BeTrue())
	})

	It("should return true if MinInstances differs", func() {
		desired := []aliv1.AliNodePool{
			{NodePoolID: "np1", MaxInstances: tea.Int64(5), MinInstances: tea.Int64(2), EnableAutoScaling: tea.Bool(true)},
		}
		upstream := map[string]aliv1.AliNodePool{
			"np1": {NodePoolID: "np1", MaxInstances: tea.Int64(5), MinInstances: tea.Int64(1), EnableAutoScaling: tea.Bool(true)},
		}
		Expect(needsUpdate(desired, upstream)).To(BeTrue())
	})

	It("should return true if DesiredSize differs", func() {
		desired := []aliv1.AliNodePool{
			{NodePoolID: "np1", DesiredSize: tea.Int64(4)},
		}
		upstream := map[string]aliv1.AliNodePool{
			"np1": {NodePoolID: "np1", DesiredSize: tea.Int64(3)},
		}
		Expect(needsUpdate(desired, upstream)).To(BeTrue())
	})

	It("should return false if EnableAutoScaling is nil", func() {
		desired := []aliv1.AliNodePool{
			{NodePoolID: "np1", MaxInstances: tea.Int64(5), MinInstances: tea.Int64(1)},
		}
		upstream := map[string]aliv1.AliNodePool{
			"np1": {NodePoolID: "np1", MaxInstances: tea.Int64(5), MinInstances: tea.Int64(1)},
		}
		Expect(needsUpdate(desired, upstream)).To(BeFalse())
	})
})

var _ = Describe("needsDelete", func() {
	It("should return false if all desired pools exist in upstream", func() {
		desired := []aliv1.AliNodePool{
			{NodePoolID: "np1"},
			{NodePoolID: "np2"},
		}
		upstream := []aliv1.AliNodePool{
			{NodePoolID: "np1"},
			{NodePoolID: "np2"},
		}
		Expect(needsDelete(desired, upstream)).To(BeFalse())
	})

	It("should return true if an upstream pool does not exist in desired", func() {
		desired := []aliv1.AliNodePool{
			{NodePoolID: "np1"},
		}
		upstream := []aliv1.AliNodePool{
			{NodePoolID: "np1"},
			{NodePoolID: "np2"},
		}
		Expect(needsDelete(desired, upstream)).To(BeTrue())
	})

	It("should return false if desired is empty and upstream is not", func() {
		desired := []aliv1.AliNodePool{}
		upstream := []aliv1.AliNodePool{
			{NodePoolID: "np1"},
			{NodePoolID: "np2"},
		}
		Expect(needsDelete(desired, upstream)).To(BeTrue())
	})

	It("should return false if both desired and upstream are empty", func() {
		desired := []aliv1.AliNodePool{}
		upstream := []aliv1.AliNodePool{}
		Expect(needsDelete(desired, upstream)).To(BeFalse())
	})
})

var _ = Describe("recordError", func() {
	var (
		handler *Handler
		config  *aliv1.AliClusterConfig
	)

	BeforeEach(func() {
		handler = &Handler{
			aliCC: aliFactory.Ali().V1().AliClusterConfig(),
		}
		config = &aliv1.AliClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-record-error",
				Namespace: "default",
			},
			Spec: aliv1.AliClusterConfigSpec{
				ClusterName: "test-record-error-cluster",
				RegionID:    "eu-central",
				ClusterType: alibaba.ManagedClusterType,
				VpcID:       "vpc-id",
				VSwitchIDs:  []string{"vswitch-id"},
			},
			Status: aliv1.AliClusterConfigStatus{},
		}
	})

	AfterEach(func() {
		Expect(test.CleanupAndWait(ctx, cl, config)).To(Succeed())
	})

	It("should not update the status if the error message is the same", func() {
		onChange := func(key string, config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
			return config, errors.New("same error")
		}
		wrapped := handler.recordError(onChange)
		config.Status.FailureMessage = "same error"

		// The object is not created in the API server.
		// The function should return before attempting to update status, so no error should occur from the client.
		_, err := wrapped("key", config)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("same error"))
	})

	It("should update status with a new error message", func() {
		createdConfig, err := aliFactory.Ali().V1().AliClusterConfig().Create(config)
		Expect(err).NotTo(HaveOccurred())

		onChange := func(key string, config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
			return config, errors.New("new error")
		}
		wrapped := handler.recordError(onChange)

		updatedConfig, err := wrapped("key", createdConfig)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("new error"))
		Expect(updatedConfig.Status.FailureMessage).To(Equal("new error"))

		fetchedConfig, err := aliFactory.Ali().V1().AliClusterConfig().Get(config.Namespace, config.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(fetchedConfig.Status.FailureMessage).To(Equal("new error"))
	})

	It("should change phase to updating if an error occurs in active phase", func() {
		createdConfig, err := aliFactory.Ali().V1().AliClusterConfig().Create(config)
		Expect(err).NotTo(HaveOccurred())

		createdConfig.Status.Phase = aliConfigActivePhase
		createdConfig, err = aliFactory.Ali().V1().AliClusterConfig().UpdateStatus(createdConfig)
		Expect(err).NotTo(HaveOccurred())

		onChange := func(key string, config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
			return config, errors.New("update error")
		}
		wrapped := handler.recordError(onChange)

		updatedConfig, err := wrapped("key", createdConfig)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("update error"))
		Expect(updatedConfig.Status.Phase).To(Equal(aliConfigUpdatingPhase))
		Expect(updatedConfig.Status.FailureMessage).To(Equal("update error"))

		fetchedConfig, err := aliFactory.Ali().V1().AliClusterConfig().Get(config.Namespace, config.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(fetchedConfig.Status.Phase).To(Equal(aliConfigUpdatingPhase))
		Expect(fetchedConfig.Status.FailureMessage).To(Equal("update error"))
	})

	It("should clear failure message if there is no error", func() {
		createdConfig, err := aliFactory.Ali().V1().AliClusterConfig().Create(config)
		Expect(err).NotTo(HaveOccurred())

		createdConfig.Status.FailureMessage = "previous error"
		createdConfig, err = aliFactory.Ali().V1().AliClusterConfig().UpdateStatus(createdConfig)
		Expect(err).NotTo(HaveOccurred())

		onChange := func(key string, config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
			return config, nil
		}
		wrapped := handler.recordError(onChange)

		updatedConfig, err := wrapped("key", createdConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedConfig.Status.FailureMessage).To(BeEmpty())

		fetchedConfig, err := aliFactory.Ali().V1().AliClusterConfig().Get(config.Namespace, config.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(fetchedConfig.Status.FailureMessage).To(BeEmpty())
	})

	It("should not update status on conflict error", func() {
		createdConfig, err := aliFactory.Ali().V1().AliClusterConfig().Create(config)
		Expect(err).NotTo(HaveOccurred())

		conflictErr := apierrors.NewConflict(schema.GroupResource{Group: "ali.cattle.io", Resource: "aliclusterconfigs"}, config.Name, errors.New("conflict"))
		onChange := func(key string, config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
			return config, conflictErr
		}
		wrapped := handler.recordError(onChange)

		_, err = wrapped("key", createdConfig)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsConflict(err)).To(BeTrue())

		// Status should not have been updated with a failure message
		fetchedConfig, err := aliFactory.Ali().V1().AliClusterConfig().Get(config.Namespace, config.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(fetchedConfig.Status.FailureMessage).To(BeEmpty())
	})

	It("should return nil if config from onChange is nil", func() {
		onChange := func(key string, config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
			return nil, nil
		}
		wrapped := handler.recordError(onChange)

		updatedConfig, err := wrapped("key", config)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedConfig).To(BeNil())
	})
})

var _ = Describe("create", func() {
	var (
		handler            *Handler
		ctrl               *gomock.Controller
		clustersClientMock *mock_services.MockClustersClientInterface
		config             *aliv1.AliClusterConfig
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)

		handler = &Handler{
			aliCC: aliFactory.Ali().V1().AliClusterConfig(),
			alibabaClients: alibabaClients{
				clustersClient: clustersClientMock,
			},
		}

		config = &aliv1.AliClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-create-cluster",
				Namespace: "default",
				UID:       "test-uid",
			},
			Spec: aliv1.AliClusterConfigSpec{
				ClusterName: "test-create-cluster-name",
				RegionID:    "eu-central",
				ClusterType: alibaba.ManagedClusterType,
				VpcID:       "vpc-id",
				VSwitchIDs:  []string{"vswitch-id"},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(test.CleanupAndWait(ctx, cl, config)).To(Succeed())
	})

	It("should handle imported cluster", func() {
		config.Spec.Imported = true
		createdConfig, err := aliFactory.Ali().V1().AliClusterConfig().Create(config)
		Expect(err).NotTo(HaveOccurred())

		updatedConfig, err := handler.create(createdConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedConfig.Status.Phase).To(Equal(aliConfigImportingPhase))

		fetchedConfig, err := aliFactory.Ali().V1().AliClusterConfig().Get(config.Namespace, config.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(fetchedConfig.Status.Phase).To(Equal(aliConfigImportingPhase))
	})

	It("should create a new cluster", func() {
		clustersClientMock.EXPECT().CreateCluster(gomock.Any(), gomock.Any()).Return(&cs.CreateClusterResponse{
			Body: &cs.CreateClusterResponseBody{
				ClusterId: tea.String("new-cluster-id"),
			},
		}, nil)

		createdConfig, err := aliFactory.Ali().V1().AliClusterConfig().Create(config)
		Expect(err).NotTo(HaveOccurred())

		updatedConfig, err := handler.create(createdConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedConfig.Spec.ClusterID).To(Equal("new-cluster-id"))
		Expect(updatedConfig.Status.Phase).To(Equal(aliConfigCreatingPhase))

		fetchedConfig, err := aliFactory.Ali().V1().AliClusterConfig().Get(config.Namespace, config.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(fetchedConfig.Spec.ClusterID).To(Equal("new-cluster-id"))
		Expect(fetchedConfig.Status.Phase).To(Equal(aliConfigCreatingPhase))
	})

	It("should return error if cluster creation fails", func() {
		apiError := errors.New("api error")
		clustersClientMock.EXPECT().CreateCluster(gomock.Any(), gomock.Any()).Return(nil, apiError)

		_, err := handler.create(config)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("api error"))
	})

	It("should not create if cluster ID already exists", func() {
		config.Spec.ClusterID = "existing-id"
		createdConfig, err := aliFactory.Ali().V1().AliClusterConfig().Create(config)
		Expect(err).NotTo(HaveOccurred())

		updatedConfig, err := handler.create(createdConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedConfig.Spec.ClusterID).To(Equal("existing-id"))
		Expect(updatedConfig.Status.Phase).To(Equal(aliConfigCreatingPhase))
	})
})

var _ = Describe("waitForCreationComplete", func() {
	var (
		handler            *Handler
		ctrl               *gomock.Controller
		clustersClientMock *mock_services.MockClustersClientInterface
		config             *aliv1.AliClusterConfig
		enqueueChan        chan struct{}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)
		enqueueChan = make(chan struct{}, 1)

		handler = &Handler{
			aliCC:        aliFactory.Ali().V1().AliClusterConfig(),
			secrets:      coreFactory.Core().V1().Secret(),
			secretsCache: coreFactory.Core().V1().Secret().Cache(),
			alibabaClients: alibabaClients{
				clustersClient: clustersClientMock,
			},
			aliEnqueueAfter: func(namespace, name string, duration time.Duration) {
				enqueueChan <- struct{}{}
			},
		}

		config = &aliv1.AliClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-wait-cluster",
				Namespace: "default",
				UID:       "test-uid-wait",
			},
			Spec: aliv1.AliClusterConfigSpec{
				ClusterID: "c12345",
			},
			Status: aliv1.AliClusterConfigStatus{
				Phase: aliConfigCreatingPhase,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
		close(enqueueChan)
		Expect(test.CleanupAndWait(ctx, cl, config)).To(Succeed())
	})

	It("should enqueue if cluster is still creating", func() {
		clusterDetailResp := &cs.DescribeClusterDetailResponse{
			Body: &cs.DescribeClusterDetailResponseBody{State: tea.String("creating")},
		}
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &config.Spec.ClusterID).Return(clusterDetailResp, nil)

		_, err := handler.waitForCreationComplete(config)
		Expect(err).NotTo(HaveOccurred())

		Eventually(enqueueChan).Should(Receive())
	})

	It("should move to active phase when cluster is running", func() {
		clusterDetailResp := &cs.DescribeClusterDetailResponse{
			Body: &cs.DescribeClusterDetailResponseBody{State: tea.String(alibaba.ClusterStatusRunning)},
		}
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &config.Spec.ClusterID).Return(clusterDetailResp, nil)

		kubeconfigResp := &cs.DescribeClusterUserKubeconfigResponse{
			Body: &cs.DescribeClusterUserKubeconfigResponseBody{
				Config: tea.String(`apiVersion: v1
clusters:
- cluster: {server: 'https://1.2.3.4:6443', certificate-authority-data: 'Cg=='}
  name: kubernetes
contexts:
- context: {cluster: kubernetes, user: ""}
  name: kubernetes
current-context: kubernetes
kind: Config
users: []`),
			},
		}
		clustersClientMock.EXPECT().DescribeClusterUserKubeconfig(gomock.Any(), &config.Spec.ClusterID, gomock.Any()).Return(kubeconfigResp, nil)

		createdConfig, err := aliFactory.Ali().V1().AliClusterConfig().Create(config)
		Expect(err).NotTo(HaveOccurred())

		updatedConfig, err := handler.waitForCreationComplete(createdConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedConfig.Status.Phase).To(Equal(aliConfigActivePhase))

		secret, err := coreFactory.Core().V1().Secret().Get(config.Namespace, config.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Data["endpoint"]).To(Equal([]byte("https://1.2.3.4:6443")))
	})

	It("should return error if cluster creation failed", func() {
		clusterDetailResp := &cs.DescribeClusterDetailResponse{
			Body: &cs.DescribeClusterDetailResponseBody{State: tea.String(alibaba.ClusterStatusFailed)},
		}
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &config.Spec.ClusterID).Return(clusterDetailResp, nil)

		_, err := handler.waitForCreationComplete(config)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("creation failed"))
	})

	It("should return error if DescribeClusterDetail fails", func() {
		apiError := errors.New("api error")
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &config.Spec.ClusterID).Return(nil, apiError)

		_, err := handler.waitForCreationComplete(config)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(apiError))
	})

	It("should return error if createCASecret fails", func() {
		clusterDetailResp := &cs.DescribeClusterDetailResponse{
			Body: &cs.DescribeClusterDetailResponseBody{State: tea.String(alibaba.ClusterStatusRunning)},
		}
		clustersClientMock.EXPECT().DescribeClusterDetail(gomock.Any(), &config.Spec.ClusterID).Return(clusterDetailResp, nil)

		apiError := errors.New("kubeconfig error")
		clustersClientMock.EXPECT().DescribeClusterUserKubeconfig(gomock.Any(), &config.Spec.ClusterID, gomock.Any()).Return(nil, apiError)

		_, err := handler.waitForCreationComplete(config)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(apiError))
	})
})

var _ = Describe("updateStatus", func() {
	var (
		handler *Handler
		config  *aliv1.AliClusterConfig
	)

	BeforeEach(func() {
		handler = &Handler{
			aliCC: aliFactory.Ali().V1().AliClusterConfig(),
		}
		config = &aliv1.AliClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-update-status",
				Namespace: "default",
			},
			Spec: aliv1.AliClusterConfigSpec{},
			Status: aliv1.AliClusterConfigStatus{
				Phase: aliConfigCreatingPhase,
			},
		}
	})

	AfterEach(func() {
		Expect(test.CleanupAndWait(ctx, cl, config)).To(Succeed())
	})

	It("should update the status of the aliclusterconfig", func() {
		createdConfig, err := aliFactory.Ali().V1().AliClusterConfig().Create(config)
		Expect(err).NotTo(HaveOccurred())

		createdConfig.Status.Phase = aliConfigActivePhase
		updatedConfig, err := handler.updateStatus(createdConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedConfig.Status.Phase).To(Equal(aliConfigActivePhase))

		fetchedConfig, err := aliFactory.Ali().V1().AliClusterConfig().Get(config.Namespace, config.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(fetchedConfig.Status.Phase).To(Equal(aliConfigActivePhase))
	})

	It("should not update if config is nil", func() {
		updatedConfig, err := handler.updateStatus(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedConfig).To(BeNil())
	})

	It("should retry on conflict", func() {
		createdConfig, err := aliFactory.Ali().V1().AliClusterConfig().Create(config)
		Expect(err).NotTo(HaveOccurred())

		// Simulate a conflict by updating the resource version in the background.
		backgroundConfig, err := aliFactory.Ali().V1().AliClusterConfig().Get(config.Namespace, config.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		backgroundConfig.Status.Phase = aliConfigUpdatingPhase
		_, _ = aliFactory.Ali().V1().AliClusterConfig().UpdateStatus(backgroundConfig)

		createdConfig.Status.Phase = aliConfigActivePhase
		_, err = handler.updateStatus(createdConfig)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("handleDeleteNodePools", func() {
	var (
		handler            *Handler
		ctrl               *gomock.Controller
		clustersClientMock *mock_services.MockClustersClientInterface
		config             *aliv1.AliClusterConfig
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)

		handler = &Handler{
			aliCC: aliFactory.Ali().V1().AliClusterConfig(),
			alibabaClients: alibabaClients{
				clustersClient: clustersClientMock,
			},
		}

		config = &aliv1.AliClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
				UID:       "test-uid",
			},
			Spec: aliv1.AliClusterConfigSpec{
				ClusterID:   "c12345",
				ClusterName: "test-cluster",
				RegionID:    "eu-central",
				ClusterType: alibaba.ManagedClusterType,
				VpcID:       "vpc-id",
				VSwitchIDs:  []string{"vswitch-id"},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(test.CleanupAndWait(ctx, cl, config)).To(Succeed())
	})

	It("should delete node pools successfully", func() {
		existingNodePools := []aliv1.AliNodePool{}
		upstreamNodePools := []aliv1.AliNodePool{
			{NodePoolID: "np1", Name: "pool1"},
		}

		clustersClientMock.EXPECT().DescribeClusterNodes(gomock.Any(), &config.Spec.ClusterID, gomock.Any()).Return(&cs.DescribeClusterNodesResponse{
			Body: &cs.DescribeClusterNodesResponseBody{
				Nodes: []*cs.DescribeClusterNodesResponseBodyNodes{},
			},
		}, nil)

		clustersClientMock.EXPECT().DeleteClusterNodePool(gomock.Any(), &config.Spec.ClusterID, &upstreamNodePools[0].NodePoolID, gomock.Any()).Return(nil, nil)

		changed, err := handler.handleDeleteNodePools(context.Background(), clustersClientMock, &config.Spec, existingNodePools, upstreamNodePools)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeTrue())
	})

	It("should not delete node pools if existing node pool has all the upstream node pools", func() {
		existingNodePools := []aliv1.AliNodePool{
			{NodePoolID: "np1", Name: "pool1"},
		}
		upstreamNodePools := []aliv1.AliNodePool{
			{NodePoolID: "np1", Name: "pool1"},
		}

		changed, err := handler.handleDeleteNodePools(context.TODO(), clustersClientMock, &config.Spec, existingNodePools, upstreamNodePools)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())
	})

	It("should return error if DeleteNodePool fails", func() {
		existingNodePools := []aliv1.AliNodePool{}
		upstreamNodePools := []aliv1.AliNodePool{
			{NodePoolID: "np1", Name: "pool1"},
		}
		apiError := errors.New("api error")
		clustersClientMock.EXPECT().DescribeClusterNodes(gomock.Any(), &config.Spec.ClusterID, gomock.Any()).Return(&cs.DescribeClusterNodesResponse{
			Body: &cs.DescribeClusterNodesResponseBody{
				Nodes: []*cs.DescribeClusterNodesResponseBodyNodes{},
			},
		}, nil)

		clustersClientMock.EXPECT().DeleteClusterNodePool(gomock.Any(), &config.Spec.ClusterID, &upstreamNodePools[0].NodePoolID, gomock.Any()).Return(nil, apiError)

		changed, err := handler.handleDeleteNodePools(context.Background(), clustersClientMock, &config.Spec, existingNodePools, upstreamNodePools)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("api error"))
		Expect(changed).To(BeTrue())
	})
})

var _ = Describe("handleUpdateNodePools", func() {
	var (
		handler            *Handler
		ctrl               *gomock.Controller
		clustersClientMock *mock_services.MockClustersClientInterface
		config             *aliv1.AliClusterConfig
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clustersClientMock = mock_services.NewMockClustersClientInterface(ctrl)

		handler = &Handler{
			aliCC: aliFactory.Ali().V1().AliClusterConfig(),
			alibabaClients: alibabaClients{
				clustersClient: clustersClientMock,
			},
		}

		config = &aliv1.AliClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
				UID:       "test-uid",
			},
			Spec: aliv1.AliClusterConfigSpec{
				ClusterID:   "c12345",
				ClusterName: "test-cluster",
				RegionID:    "eu-central",
				ClusterType: alibaba.ManagedClusterType,
				VpcID:       "vpc-id",
				VSwitchIDs:  []string{"vswitch-id"},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(test.CleanupAndWait(ctx, cl, config)).To(Succeed())
	})

	It("should update node pools autoscaling successfully", func() {
		updateQueue := []aliv1.AliNodePool{
			{NodePoolID: "np1", Name: "pool1", EnableAutoScaling: tea.Bool(true), MaxInstances: tea.Int64(6), MinInstances: tea.Int64(1)},
		}
		upstreamNodePoolConfigMap := map[string]aliv1.AliNodePool{
			"np1": {NodePoolID: "np1", Name: "pool1", EnableAutoScaling: tea.Bool(true), MaxInstances: tea.Int64(5), MinInstances: tea.Int64(1)},
		}
		clustersClientMock.EXPECT().ModifyClusterNodePool(gomock.Any(), &config.Spec.ClusterID, &updateQueue[0].NodePoolID, gomock.Any()).Return(nil, nil)

		changed, err := handler.handleUpdateNodePools(context.Background(), &config.Spec, updateQueue, upstreamNodePoolConfigMap)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeTrue())
	})

	It("should update node pools desired size successfully", func() {
		updateQueue := []aliv1.AliNodePool{
			{NodePoolID: "np1", Name: "pool1", DesiredSize: tea.Int64(4)},
		}
		upstreamNodePoolConfigMap := map[string]aliv1.AliNodePool{
			"np1": {NodePoolID: "np1", Name: "pool1", DesiredSize: tea.Int64(3)},
		}
		clustersClientMock.EXPECT().ModifyClusterNodePool(gomock.Any(), &config.Spec.ClusterID, &updateQueue[0].NodePoolID, gomock.Any()).Return(nil, nil)

		changed, err := handler.handleUpdateNodePools(context.Background(), &config.Spec, updateQueue, upstreamNodePoolConfigMap)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeTrue())
	})

	It("should skip update if autoscaling configuration mismatch", func() {
		updateQueue := []aliv1.AliNodePool{
			{NodePoolID: "np1", Name: "pool1", EnableAutoScaling: tea.Bool(true), MaxInstances: tea.Int64(6), MinInstances: tea.Int64(1)},
		}
		upstreamNodePoolConfigMap := map[string]aliv1.AliNodePool{
			"np1": {NodePoolID: "np1", Name: "pool1", EnableAutoScaling: tea.Bool(false), MaxInstances: tea.Int64(5), MinInstances: tea.Int64(1)},
		}

		changed, err := handler.handleUpdateNodePools(context.Background(), &config.Spec, updateQueue, upstreamNodePoolConfigMap)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())
	})

	It("should return error if UpdateNodePoolAutoScalingConfig fails", func() {
		updateQueue := []aliv1.AliNodePool{
			{NodePoolID: "np1", Name: "pool1", EnableAutoScaling: tea.Bool(true), MaxInstances: tea.Int64(6), MinInstances: tea.Int64(1)},
		}
		upstreamNodePoolConfigMap := map[string]aliv1.AliNodePool{
			"np1": {NodePoolID: "np1", Name: "pool1", EnableAutoScaling: tea.Bool(true), MaxInstances: tea.Int64(5), MinInstances: tea.Int64(1)},
		}
		apiError := errors.New("api error")
		clustersClientMock.EXPECT().ModifyClusterNodePool(gomock.Any(), &config.Spec.ClusterID, &updateQueue[0].NodePoolID, gomock.Any()).Return(nil, apiError)

		changed, err := handler.handleUpdateNodePools(context.Background(), &config.Spec, updateQueue, upstreamNodePoolConfigMap)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("api error"))
		Expect(changed).To(BeFalse())
	})

	It("should return error if UpdateNodePoolDesiredSize fails", func() {
		updateQueue := []aliv1.AliNodePool{
			{NodePoolID: "np1", Name: "pool1", DesiredSize: tea.Int64(4)},
		}
		upstreamNodePoolConfigMap := map[string]aliv1.AliNodePool{
			"np1": {NodePoolID: "np1", Name: "pool1", DesiredSize: tea.Int64(3)},
		}
		apiError := errors.New("api error")
		clustersClientMock.EXPECT().ModifyClusterNodePool(gomock.Any(), &config.Spec.ClusterID, &updateQueue[0].NodePoolID, gomock.Any()).Return(nil, apiError)

		changed, err := handler.handleUpdateNodePools(context.Background(), &config.Spec, updateQueue, upstreamNodePoolConfigMap)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("api error"))
		Expect(changed).To(BeFalse())
	})
})

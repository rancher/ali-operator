package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	cs "github.com/rancher/muchang/cs/client"
	"github.com/rancher/muchang/utils/tea"

	"github.com/rancher/ali-operator/pkg/alibaba"
	"github.com/rancher/ali-operator/pkg/alibaba/services"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	alicontrollers "github.com/rancher/ali-operator/pkg/generated/controllers/ali.cattle.io/v1"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

const (
	aliClusterConfigKind     = "AliClusterConfig"
	aliConfigCreatingPhase   = "creating"
	aliConfigNotCreatedPhase = ""
	aliConfigActivePhase     = "active"
	aliConfigUpdatingPhase   = "updating"
	aliConfigImportingPhase  = "importing"
	controllerName           = "ali-controller"
	controllerRemoveName     = "ali-controller-remove"
	enqueuePeriod            = 30 * time.Second
)

type alibabaClients struct {
	clustersClient services.ClustersClientInterface
}

type Handler struct {
	aliCC           alicontrollers.AliClusterConfigClient
	secrets         wranglerv1.SecretClient
	secretsCache    wranglerv1.SecretCache
	aliEnqueueAfter func(namespace, name string, duration time.Duration)
	aliEnqueue      func(namespace, name string)
	alibabaClients  alibabaClients
}

func Register(
	ctx context.Context,
	secrets wranglerv1.SecretController,
	ali alicontrollers.AliClusterConfigController) {
	controller := &Handler{
		aliCC:           ali,
		secretsCache:    secrets.Cache(),
		secrets:         secrets,
		aliEnqueue:      ali.Enqueue,
		aliEnqueueAfter: ali.EnqueueAfter,
	}

	// Register handlers
	ali.OnChange(ctx, controllerName, controller.recordError(controller.OnAliConfigChanged))
	ali.OnRemove(ctx, controllerRemoveName, controller.OnAliConfigRemoved)
}

func (h *Handler) OnAliConfigChanged(_ string, config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
	if config == nil || config.DeletionTimestamp != nil {
		return nil, nil
	}

	if err := h.getAlibabaClients(config); err != nil {
		return config, fmt.Errorf("error getting Alibaba clients: %w", err)
	}

	switch config.Status.Phase {
	case aliConfigImportingPhase:
		return h.importCluster(config)
	case aliConfigNotCreatedPhase:
		return h.create(config)
	case aliConfigCreatingPhase:
		return h.waitForCreationComplete(config)
	case aliConfigActivePhase, aliConfigUpdatingPhase:
		return h.checkAndUpdate(config)
	default:
		return config, fmt.Errorf("invalid phase: %v", config.Status.Phase)
	}
}

// recordError writes the error return by onChange to the failureMessage field on status. If there is no error, then
// empty string will be written to status
func (h *Handler) recordError(onChange func(key string, config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error)) func(key string, config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
	return func(key string, config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
		var err error
		var message string
		config, err = onChange(key, config)
		if config == nil {
			// Ali cluster config is likely deleting
			return config, err
		}
		if err != nil {
			if apierrors.IsConflict(err) {
				// conflict error means the config is updated by rancher controller
				// the changes which needs to be done by the operator controller will be handled in next
				// reconcile call
				logrus.Debugf("Error updating aliclusterconfig: %s", err.Error())
				return config, err
			}

			message = err.Error()
		}

		if config.Status.FailureMessage == message {
			return config, err
		}

		config = config.DeepCopy()
		if message != "" && config.Status.Phase == aliConfigActivePhase {
			// can assume an update is failing
			config.Status.Phase = aliConfigUpdatingPhase
		}
		config.Status.FailureMessage = message

		var recordErr error
		config, recordErr = h.aliCC.UpdateStatus(config)
		if recordErr != nil {
			logrus.Errorf("Error recording alicc [%s (id: %s)] failure message: %s", config.Spec.ClusterName, config.Name, recordErr.Error())
		}
		return config, err
	}
}

func (h *Handler) OnAliConfigRemoved(_ string, config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if config.Spec.Imported {
		logrus.Infof("Cluster [%s] is imported, will not delete ACK cluster", config.Name)
		return config, nil
	}

	if err := h.getAlibabaClients(config); err != nil {
		return config, fmt.Errorf("error getting Alibaba clients: %w", err)
	}

	if config.Status.Phase == aliConfigNotCreatedPhase {
		logrus.Warnf("Cluster [%s] never advanced to creating status, will not delete ACK cluster", config.Name)
		return config, nil
	}

	_, err := h.alibabaClients.clustersClient.DescribeClusterDetail(ctx, &config.Spec.ClusterID)
	if alibaba.IsNotFound(err) {
		logrus.Infof("Cluster %v , region %v already removed", config.Spec.ClusterName, config.Spec.RegionID)
		return config, nil
	} else if err != nil {
		logrus.Errorf("Get Cluster %v error: %+v", config.Spec.ClusterName, err)
		return config, err
	}

	logrus.Infof("Removing cluster %v , region %v", config.Spec.ClusterName, config.Spec.RegionID)
	if _, err := h.alibabaClients.clustersClient.DeleteCluster(ctx, &config.Spec.ClusterID, &cs.DeleteClusterRequest{}); err != nil {
		logrus.Debugf("Error deleting cluster %s: %v", config.Spec.ClusterName, err)
		return config, err
	}
	logrus.Infof("Successfully sent request to delete cluster %v , region %v", config.Spec.ClusterName, config.Spec.RegionID)

	return config, nil
}

func (h *Handler) importCluster(config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logrus.Infof("Importing config for cluster [%s (id: %s)]", config.Spec.ClusterName, config.Name)

	clusterResp, err := h.alibabaClients.clustersClient.DescribeClusterDetail(ctx, &config.Spec.ClusterID)
	if err != nil {
		return config, err
	}
	if clusterResp == nil || clusterResp.Body == nil || clusterResp.Body.State == nil || *clusterResp.Body.State != alibaba.ClusterStatusRunning {
		state := "unknown"
		if clusterResp != nil && clusterResp.Body != nil && clusterResp.Body.State != nil {
			state = *clusterResp.Body.State
		}
		return config, fmt.Errorf("the current cluster status is %s. Please wait for the cluster to become %s", state, alibaba.ClusterStatusRunning)
	}

	if err := h.createCASecret(ctx, config); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return config, err
		}
	}

	config.Status.Phase = aliConfigActivePhase
	return h.aliCC.UpdateStatus(config)
}

func (h *Handler) create(config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if config.Spec.Imported {
		config = config.DeepCopy()
		config.Status.Phase = aliConfigImportingPhase
		return h.aliCC.UpdateStatus(config)
	}

	if config.Spec.ClusterID == "" {
		clusterID, err := alibaba.Create(ctx, h.alibabaClients.clustersClient, &config.Spec)
		if err != nil {
			return config, err
		}
		config.Spec.ClusterID = clusterID
	}
	configUpdate := config.DeepCopy()
	configUpdate, err := h.aliCC.Update(configUpdate)
	if err != nil {
		return config, err
	}
	config = configUpdate.DeepCopy()
	config.Status.Phase = aliConfigCreatingPhase
	config, err = h.aliCC.UpdateStatus(config)
	logrus.Infof("Cluster id:%s for cluster:%s", config.Spec.ClusterID, config.Spec.ClusterName)
	return config, err
}

func (h *Handler) waitForCreationComplete(config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clusterResp, err := h.alibabaClients.clustersClient.DescribeClusterDetail(ctx, &config.Spec.ClusterID)
	if err != nil {
		return config, err
	}
	if clusterResp == nil || clusterResp.Body == nil {
		return config, fmt.Errorf("create cluster error: got the cluster as nil, indicating no cluster information is available")
	}
	if *clusterResp.Body.State == alibaba.ClusterStatusFailed {
		return config, fmt.Errorf("creation failed for cluster %v", config.Spec.ClusterName)
	}
	if *clusterResp.Body.State == alibaba.ClusterStatusRunning {
		if err := h.createCASecret(ctx, config); err != nil {
			return config, err
		}
		logrus.Infof("Cluster %v is running", config.Spec.ClusterName)
		config = config.DeepCopy()
		config.Status.Phase = aliConfigActivePhase
		return h.aliCC.UpdateStatus(config)
	}

	logrus.Infof("Waiting for cluster [%s] to finish creating", config.Name)
	h.aliEnqueueAfter(config.Namespace, config.Name, enqueuePeriod)
	return config, nil
}

func (h *Handler) checkAndUpdate(config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cluster, err := alibaba.GetCluster(ctx, h.alibabaClients.clustersClient, config.Spec.ClusterID)
	if err != nil {
		logrus.Errorf("update cluster error: %v", err)
		return config, err
	}

	alibaba.SyncConfigSpecClusterFieldsWithUpstream(&config.Spec, cluster)

	if config.Status.UpgradeTaskID != "" {
		taskInfoResp, err := h.alibabaClients.clustersClient.DescribeTaskInfo(ctx, &config.Status.UpgradeTaskID)
		if err != nil {
			return config, fmt.Errorf("failed to describe task info for cluster %s: %w", config.Spec.ClusterID, err)
		}
		if taskInfoResp == nil || taskInfoResp.Body == nil {
			logrus.Debugf("Received empty task info for cluster %s, err: received empty task info", config.Spec.ClusterID)
		} else {
			taskInfo := taskInfoResp.Body
			if taskInfo.State != nil {
				switch *taskInfo.State {
				case alibaba.UpdateK8sRunningStatus:
					logrus.Infof("Cluster %s in region %s is being upgraded", config.Spec.ClusterName, config.Spec.RegionID)
					return h.enqueueUpdate(config, enqueuePeriod)
				case alibaba.UpdateK8sFailStatus:
					if taskInfo.Error == nil || taskInfo.Error.Message == nil {
						return config, fmt.Errorf("update cluster %s failed: error message is missing", config.Spec.ClusterID)
					}
					errMsg := fmt.Sprintf(`{"%s":"%s"}`, alibaba.UpdateK8SError, *taskInfo.Error.Message)
					return config, fmt.Errorf("update cluster %s failed: %s", config.Spec.ClusterID, errMsg)
				case alibaba.UpdateK8sSuccessStatus:
					config = config.DeepCopy()
					config.Status.UpgradeTaskID = ""
					return h.aliCC.UpdateStatus(config)
				default:
					logrus.Warnf("Received unknown task[%s] status %s for cluster %s", tea.StringValue(taskInfo.TaskId), tea.StringValue(taskInfo.State), config.Spec.ClusterName)
				}
			}
		}
	}

	// waiting for cluster to come in running state
	clusterState := tea.StringValue(cluster.State)
	if clusterState == "" {
		config = config.DeepCopy()
		config.Status.FailureMessage = fmt.Sprintf("Upstream cluster %s in region %s is in unknown state", config.Spec.ClusterName, config.Spec.RegionID)
		config.Status.Phase = aliConfigUpdatingPhase
		return h.aliCC.UpdateStatus(config)
	}

	if clusterState != alibaba.ClusterStatusRunning {
		logrus.Infof("Cluster is in %s state, waiting for the cluster to come in %s state", clusterState, alibaba.ClusterStatusRunning)
		return h.enqueueUpdate(config, enqueuePeriod)
	}

	nodePools, err := alibaba.GetNodePools(ctx, h.alibabaClients.clustersClient, &config.Spec)
	if err != nil {
		return config, err
	}
	for _, np := range nodePools {
		if np == nil || np.NodepoolInfo == nil || np.NodepoolInfo.Name == nil {
			logrus.Warn("Update cluster: no nodepool information is available")
			continue
		}
		if np.Status == nil || np.Status.State == nil {
			logrus.Warnf("Update cluster: nodepool %s is in unknown state", *np.NodepoolInfo.Name)
			continue
		}
		if alibaba.WaitForNodePool(np.Status.State) {
			logrus.Infof("Waiting for cluster [%s] to sync nodepool [%s] state [%s]", config.Spec.ClusterName, *np.NodepoolInfo.Name, *np.Status.State)
			return h.enqueueUpdate(config, enqueuePeriod)
		}
	}

	if config.Spec.KubernetesVersion != tea.StringValue(cluster.CurrentVersion) {
		if config.Status.Phase != aliConfigUpdatingPhase {
			return h.enqueueUpdate(config, enqueuePeriod)
		}
		taskID, err := alibaba.UpgradeCluster(ctx, h.alibabaClients.clustersClient, &config.Spec)
		if err != nil {
			updateErr := fmt.Errorf(`{"%s":"%s"}`, alibaba.UpdateK8SVersionApiError, err.Error())
			return config, updateErr
		}
		config = config.DeepCopy()
		config.Status.UpgradeTaskID = taskID
		return h.updateStatus(config)
	}

	return h.updateUpstreamClusterState(config)
}

func (h *Handler) getAlibabaClients(config *aliv1.AliClusterConfig) error {
	credentials, err := alibaba.GetSecrets(h.secretsCache, &config.Spec)
	if err != nil {
		return fmt.Errorf("error getting credentials: %w", err)
	}

	clustersClient, err := services.NewClustersClient(credentials, config.Spec.RegionID)
	if err != nil {
		return fmt.Errorf("error creating client secret credential: %w", err)
	}

	h.alibabaClients = alibabaClients{
		clustersClient: clustersClient,
	}

	return nil
}

// createCASecret creates a secret containing ca and endpoint. These can be used to create a kubeconfig via
// the go sdk
func (h *Handler) createCASecret(ctx context.Context, config *aliv1.AliClusterConfig) error {

	kubeConfigResp, err := h.alibabaClients.clustersClient.DescribeClusterUserKubeconfig(ctx, &config.Spec.ClusterID, &cs.DescribeClusterUserKubeconfigRequest{})
	if err != nil {
		return err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(*kubeConfigResp.Body.Config))
	if err != nil {
		return err
	}

	_, err = h.secrets.Create(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      config.Name,
				Namespace: config.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: aliv1.SchemeGroupVersion.String(),
						Kind:       aliClusterConfigKind,
						UID:        config.UID,
						Name:       config.Name,
					},
				},
			},
			Data: map[string][]byte{
				"endpoint": []byte(restConfig.Host),
				"ca":       []byte(base64.StdEncoding.EncodeToString(restConfig.CAData)),
			},
		})
	if apierrors.IsAlreadyExists(err) {
		logrus.Debugf("CA secret [%s] already exists, ignoring", config.Name)
		return nil
	}
	return err
}

// enqueueUpdate enqueues the config if it is already in the updating phase. Otherwise, the
// phase is updated to "updating".
func (h *Handler) enqueueUpdate(config *aliv1.AliClusterConfig, period time.Duration) (*aliv1.AliClusterConfig, error) {
	if config.Status.Phase == aliConfigUpdatingPhase {
		h.aliEnqueueAfter(config.Namespace, config.Name, period)
		return config, nil
	}

	config = config.DeepCopy()
	config.Status.Phase = aliConfigUpdatingPhase
	return h.aliCC.UpdateStatus(config)
}

// updateStatus updates the status of config with retry
func (h *Handler) updateStatus(config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
	if config == nil {
		return config, nil
	}
	configStatus := config.Status.DeepCopy()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		config, err = h.aliCC.Get(config.Namespace, config.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		config = config.DeepCopy()
		config.Status = *configStatus
		config, err = h.aliCC.UpdateStatus(config)
		return err
	})
	return config, err
}

// updateUpstreamClusterState sync config to upstream cluster
func (h *Handler) updateUpstreamClusterState(config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changed, err := h.updateNodePools(ctx, config)
	if err != nil {
		return config, err
	}
	if changed {
		return h.enqueueUpdate(config, 0)
	}

	// no new updates, set to active
	if config.Status.Phase != aliConfigActivePhase {
		logrus.Infof("cluster [%s] finished updating", config.Name)
		config = config.DeepCopy()
		config.Status.Phase = aliConfigActivePhase
		return h.aliCC.UpdateStatus(config)
	}

	return config, nil
}

func (h *Handler) updateNodePools(
	ctx context.Context,
	config *aliv1.AliClusterConfig,
) (bool, error) {
	client := h.alibabaClients.clustersClient

	upstreamNodePools, err := alibaba.GetNodePools(ctx, client, &config.Spec)
	if err != nil {
		return false, err
	}
	upstreamNodePoolsConfig := alibaba.ToNodePoolConfig(upstreamNodePools)

	// lookup maps
	nodePoolNameKeyMap := make(map[string]aliv1.AliNodePool)
	upstreamNodePoolConfigMap := make(map[string]aliv1.AliNodePool)
	for _, np := range upstreamNodePoolsConfig {
		upstreamNodePoolConfigMap[np.NodePoolID] = np
		nodePoolNameKeyMap[np.Name] = np
	}

	// fix NodePoolIDs
	for i, npConfig := range config.Spec.NodePools {
		if nodePool, ok := nodePoolNameKeyMap[npConfig.Name]; ok {
			config.Spec.NodePools[i].NodePoolID = nodePool.NodePoolID
		}
	}

	// classify desired pools
	var (
		updateQueue []aliv1.AliNodePool
		createQueue []aliv1.AliNodePool
	)
	for _, npConfig := range config.Spec.NodePools {
		if npConfig.NodePoolID != "" {
			updateQueue = append(updateQueue, *npConfig.DeepCopy())
		} else {
			createQueue = append(createQueue, *npConfig.DeepCopy())
		}
	}

	createNeeded := len(createQueue) > 0
	updateNeeded := needsUpdate(updateQueue, upstreamNodePoolConfigMap)
	deleteNeeded := needsDelete(updateQueue, upstreamNodePoolsConfig)

	if !createNeeded && !updateNeeded && !deleteNeeded {
		return false, nil
	}
	if config.Status.Phase != aliConfigUpdatingPhase {
		return true, err
	}

	changed := false

	if createNeeded {
		changed, err := h.handleCreateNodePools(ctx, client, &config.Spec, createQueue)
		if err != nil {
			return changed, err
		}
		if changed {
			return changed, nil
		}
	}

	if updateNeeded {
		changed, err := h.handleUpdateNodePools(ctx, &config.Spec, updateQueue, upstreamNodePoolConfigMap)
		if err != nil {
			return changed, err
		}
		if changed {
			return changed, nil
		}
	}

	if deleteNeeded {
		changed, err := h.handleDeleteNodePools(ctx, client, &config.Spec, updateQueue, upstreamNodePoolsConfig)
		if err != nil {
			return changed, err
		}
		if changed {
			return changed, nil
		}
	}

	return changed, nil
}

// handleCreateNodePools creates nodepools and updates IDs in configSpec.
func (h *Handler) handleCreateNodePools(
	ctx context.Context,
	client services.ClustersClientInterface,
	configSpec *aliv1.AliClusterConfigSpec,
	nodePools []aliv1.AliNodePool,
) (bool, error) {
	var failed []string
	changed := false
	for _, np := range nodePools {
		logrus.Infof("Creating nodepool [%s] for cluster [%s]", np.Name, configSpec.ClusterName)
		c, err := alibaba.CreateNodePool(ctx, client, configSpec, &np)
		if err != nil {
			failed = append(failed, fmt.Sprintf("nodepool %s create error: %s", np.Name, err.Error()))
			continue
		}
		changed = true
		logrus.Infof("Nodepool [%s] created with id [%s] in cluster [%s]",
			np.Name, tea.StringValue(c.NodepoolId), configSpec.ClusterName)
	}
	if len(failed) > 0 {
		return changed, fmt.Errorf("%s", strings.Join(failed, ";"))
	}
	return changed, nil
}

// handleUpdateNodePools scales nodepools up/down if needed.
func (h *Handler) handleUpdateNodePools(
	ctx context.Context,
	configSpec *aliv1.AliClusterConfigSpec,
	updateQueue []aliv1.AliNodePool,
	upstream map[string]aliv1.AliNodePool,
) (bool, error) {
	var failed []string
	changed := false
	for _, np := range updateQueue {
		unp, ok := upstream[np.NodePoolID]
		if !ok {
			continue
		}
		if tea.BoolValue(unp.EnableAutoScaling) != tea.BoolValue(np.EnableAutoScaling) {
			logrus.Warnf("Skipping update for nodepool [%s], scaling configuration mismatch in cluster [%s]", unp.Name, configSpec.ClusterName)
			continue
		}
		if np.EnableAutoScaling != nil && np.MinInstances != nil && np.MaxInstances != nil &&
			unp.EnableAutoScaling != nil && tea.BoolValue(unp.EnableAutoScaling) && tea.BoolValue(np.EnableAutoScaling) &&
			(tea.Int64Value(np.MaxInstances) != tea.Int64Value(unp.MaxInstances) ||
				tea.Int64Value(np.MinInstances) != tea.Int64Value(unp.MinInstances)) {

			logrus.Infof("Updating nodepool [%s] in cluster [%s] from maxInstances:[%d] minInstance: [%d] to maxInstances:[%d] minInstance: [%d]",
				np.Name, configSpec.ClusterName, tea.Int64Value(unp.MaxInstances), tea.Int64Value(unp.MinInstances), tea.Int64Value(np.MaxInstances), tea.Int64Value(np.MinInstances))
			err := alibaba.UpdateNodePoolAutoScalingConfig(ctx, h.alibabaClients.clustersClient, configSpec.ClusterID, np.NodePoolID, np.MaxInstances, np.MinInstances)
			if err != nil {
				failed = append(failed, fmt.Sprintf("nodepool %s autoscaling update error: %s", np.Name, err.Error()))
				continue
			}
			changed = true
		} else if np.DesiredSize != nil && unp.DesiredSize != nil && tea.Int64Value(np.DesiredSize) != tea.Int64Value(unp.DesiredSize) {
			logrus.Infof("Updating desired size for nodepool [%s] in cluster [%s] from %d to %d",
				np.Name, configSpec.ClusterName, tea.Int64Value(unp.DesiredSize), tea.Int64Value(np.DesiredSize))
			err := alibaba.UpdateNodePoolDesiredSize(ctx, h.alibabaClients.clustersClient, configSpec.ClusterID, np.NodePoolID, np.DesiredSize)
			if err != nil {
				failed = append(failed, fmt.Sprintf("nodepool %s desired size update error: %s", np.Name, err.Error()))
				continue
			}
			changed = true
		}
	}
	if len(failed) > 0 {
		return changed, fmt.Errorf("%s", strings.Join(failed, ";"))
	}
	return changed, nil
}

// handleDeleteNodePools deletes upstream pools not present in desired config.
func (h *Handler) handleDeleteNodePools(
	ctx context.Context,
	client services.ClustersClientInterface,
	configSpec *aliv1.AliClusterConfigSpec,
	existingNodePools []aliv1.AliNodePool,
	upstreamPools []aliv1.AliNodePool,
) (bool, error) {
	updatedIDMap := make(map[string]struct{})
	for _, pool := range existingNodePools {
		updatedIDMap[pool.NodePoolID] = struct{}{}
	}
	for _, np := range upstreamPools {
		if _, ok := updatedIDMap[np.NodePoolID]; ok {
			continue
		}
		logrus.Infof("Deleting node pool [%s] (id %s) for from cluster [%s]", np.Name, np.NodePoolID, configSpec.ClusterName)
		err := alibaba.DeleteNodePool(ctx, client, configSpec.ClusterID, &np)
		if err != nil {
			return true, fmt.Errorf("nodepool %s delete error: %s", np.NodePoolID, err.Error())
		}
		// deletion needs to be done one by one
		return true, nil
	}

	return false, nil
}

func needsUpdate(desired []aliv1.AliNodePool, upstream map[string]aliv1.AliNodePool) bool {
	for _, np := range desired {
		unp, ok := upstream[np.NodePoolID]
		if !ok {
			continue
		}
		// AutoScaling mismatch
		if np.EnableAutoScaling != nil && unp.EnableAutoScaling != nil {
			if tea.BoolValue(np.EnableAutoScaling) && tea.BoolValue(unp.EnableAutoScaling) {
				if tea.Int64Value(np.MaxInstances) != tea.Int64Value(unp.MaxInstances) ||
					tea.Int64Value(np.MinInstances) != tea.Int64Value(unp.MinInstances) {
					return true
				}
			}
		}
		// DesiredSize mismatch
		if np.DesiredSize != nil && unp.DesiredSize != nil &&
			tea.Int64Value(np.DesiredSize) != tea.Int64Value(unp.DesiredSize) {
			return true
		}
	}
	return false
}

// needsDelete checks if upstream has pools that aren’t in desired
func needsDelete(desired []aliv1.AliNodePool, upstream []aliv1.AliNodePool) bool {
	desiredMap := make(map[string]struct{})
	for _, d := range desired {
		desiredMap[d.NodePoolID] = struct{}{}
	}
	for _, np := range upstream {
		if _, ok := desiredMap[np.NodePoolID]; !ok {
			return true
		}
	}
	return false
}

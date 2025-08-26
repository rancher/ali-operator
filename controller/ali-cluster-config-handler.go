package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	cs "github.com/alibabacloud-go/cs-20151215/v5/client"

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
	enqueuePeriod            = 30
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
				logrus.Debugf("Error updating aksclusterconfig: %s", err.Error())
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

	return config, nil
}

func (h *Handler) importCluster(_ *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
	return &aliv1.AliClusterConfig{}, nil
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
		if err := alibaba.Create(ctx, h.alibabaClients.clustersClient, &config.Spec); err != nil {
			return config, err
		}
	}

	configUpdate := config.DeepCopy()
	configUpdate, err := h.aliCC.Update(configUpdate)
	if err != nil {
		return config, err
	}

	config = configUpdate.DeepCopy()
	config.Status.Phase = aliConfigCreatingPhase
	config, err = h.aliCC.UpdateStatus(config)
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
	h.aliEnqueueAfter(config.Namespace, config.Name, enqueuePeriod*time.Second)
	return config, nil
}

func (h *Handler) checkAndUpdate(_ *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
	return &aliv1.AliClusterConfig{}, nil
}

func (h *Handler) getAlibabaClients(config *aliv1.AliClusterConfig) error {
	credentials, err := alibaba.GetSecrets(h.secrets, &config.Spec)
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

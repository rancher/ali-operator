package controller

import (
	"context"

	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	alicontrollers "github.com/rancher/ali-operator/pkg/generated/controllers/ali.cattle.io/v1"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
)

const (
	controllerName       = "ali-controller"
	controllerRemoveName = "ali-controller-remove"
)

type Handler struct {
	aliCC        alicontrollers.AliClusterConfigClient
	secrets      wranglerv1.SecretClient
	secretsCache wranglerv1.SecretCache
}

func Register(
	ctx context.Context,
	secrets wranglerv1.SecretController,
	ali alicontrollers.AliClusterConfigController) {
	controller := &Handler{
		aliCC:        ali,
		secretsCache: secrets.Cache(),
		secrets:      secrets,
	}

	// Register handlers
	ali.OnChange(ctx, controllerName, controller.OnAliConfigChanged)
}

func (h *Handler) OnAliConfigChanged(_ string, config *aliv1.AliClusterConfig) (*aliv1.AliClusterConfig, error) {
	return &aliv1.AliClusterConfig{}, nil
}

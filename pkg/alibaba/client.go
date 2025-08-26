package alibaba

import (
	"fmt"
	"strings"

	"github.com/rancher/ali-operator/pkg/alibaba/services"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultNamespace = "default"

func GetSecrets(secretClient wranglerv1.SecretClient, spec *aliv1.AliClusterConfigSpec) (*services.Credentials, error) {
	var cred services.Credentials

	if spec.AlibabaCredentialSecret == "" {
		return nil, fmt.Errorf("secret name not provided")
	}

	ns, id := ParseSecretName(spec.AlibabaCredentialSecret)
	secret, err := secretClient.Get(ns, id, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting credential secret [%s] in namespace [%s]: %w", id, ns, err)
	}

	accessKeyID := secret.Data["alibabacredentialConfig-accessKeyId"]
	accessKeySecret := secret.Data["alibabacredentialConfig-accessKeySecret"]

	cannotBeNilError := "field [alibabacredentialConfig-%s] must be provided in cloud credential"
	if accessKeyID == nil {
		return nil, fmt.Errorf(cannotBeNilError, "accessKeyId")
	}
	if accessKeySecret == nil {
		return nil, fmt.Errorf(cannotBeNilError, "accessKeySecret")
	}

	cred.AccessKeyID = string(accessKeyID)
	cred.AccessKeySecret = string(accessKeySecret)
	return &cred, nil
}

func ParseSecretName(ref string) (namespace string, name string) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) == 1 {
		return defaultNamespace, parts[0]
	}
	return parts[0], parts[1]
}

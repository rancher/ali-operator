package main

import (
	"fmt"
	"os"

	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	controllergen "github.com/rancher/wrangler/v3/pkg/controller-gen"
	"github.com/rancher/wrangler/v3/pkg/controller-gen/args"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/rancher/wrangler/v3/pkg/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	err := os.Unsetenv("GOPATH")
	if err != nil {
		panic(err)
	}

	controllergen.Run(args.Options{
		OutputPackage: "github.com/rancher/ali-operator/pkg/generated",
		Boilerplate:   "pkg/codegen/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"ali.cattle.io": {
				Types: []interface{}{
					"./pkg/apis/ali.cattle.io/v1",
				},
				GenerateTypes: true,
			},
		},
	})

	aliClusterConfig := newCRD(&aliv1.AliClusterConfig{}, func(c crd.CRD) crd.CRD {
		c.ShortNames = []string{"alicc"}
		return c
	})

	obj, err := aliClusterConfig.ToCustomResourceDefinition()
	if err != nil {
		panic(err)
	}

	obj.(*unstructured.Unstructured).SetAnnotations(map[string]string{
		"helm.sh/resource-policy": "keep",
	})

	aliCCYaml, err := yaml.Export(obj)
	if err != nil {
		panic(err)
	}

	if err := saveCRDYaml("ali-operator-crd", string(aliCCYaml)); err != nil {
		panic(err)
	}

	fmt.Printf("obj yaml: %s", aliCCYaml)
}

func newCRD(obj interface{}, customize func(crd.CRD) crd.CRD) crd.CRD {
	crd := crd.CRD{
		GVK: schema.GroupVersionKind{
			Group:   "ali.cattle.io",
			Version: "v1",
		},
		Status:       true,
		SchemaObject: obj,
	}
	if customize != nil {
		crd = customize(crd)
	}
	return crd
}

func saveCRDYaml(name, yaml string) error {
	filename := fmt.Sprintf("./charts/%s/templates/crds.yaml", name)
	save, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer func() {
		if err := save.Close(); err != nil {
			fmt.Printf("failed to close: %v\n", err)
		}
	}()

	if err := save.Chmod(0755); err != nil {
		return err
	}

	if _, err := fmt.Fprint(save, yaml); err != nil {
		return err
	}

	return nil
}

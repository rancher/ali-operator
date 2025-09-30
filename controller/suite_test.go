package controller

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	alicontrollers "github.com/rancher/ali-operator/pkg/generated/controllers/ali.cattle.io"
	"github.com/rancher/ali-operator/pkg/test"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	testEnv     *envtest.Environment
	cfg         *rest.Config
	cl          client.Client
	coreFactory *core.Factory
	aliFactory  *alicontrollers.Factory

	ctx    context.Context
	cancel context.CancelFunc
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ali Operator Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.Background())

	By("bootstrapping test environment")
	var err error
	testEnv = &envtest.Environment{}
	cfg, cl, err = test.StartEnvTest(testEnv)
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	Expect(cl).NotTo(BeNil())

	coreFactory, err = core.NewFactoryFromConfig(cfg)
	Expect(err).NotTo(HaveOccurred())
	Expect(coreFactory).NotTo(BeNil())

	aliFactory, err = alicontrollers.NewFactoryFromConfig(cfg)
	Expect(err).NotTo(HaveOccurred())
	Expect(aliFactory).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	Expect(test.StopEnvTest(testEnv)).To(Succeed())
})

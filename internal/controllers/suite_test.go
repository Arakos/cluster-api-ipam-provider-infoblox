/*
Copyright 2023 Deutsche Telekom AG.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"github.com/telekom/cluster-api-ipam-provider-infoblox/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.
//
// The suite deliberately provides infrastructure only: an API server, a scheme, and two clients.
// No controller runs here and no mock is set up here. Every spec builds the reconciler it exercises
// with its own dependencies and calls Reconcile directly, which keeps the setup a spec needs
// visible in the spec itself.

var (
	cfg     *rest.Config
	testEnv *envtest.Environment
	ctx     context.Context
	cancel  context.CancelFunc

	// apiClient talks straight to the API server. Use it for arranging fixtures and for assertions.
	apiClient client.Client
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.Background())
	ctx = logf.IntoContext(ctx, logf.Log)

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			filepath.Join("..", "..", "config", "crd", "test"),
		},
		ErrorIfCRDPathMissing:   true,
		ControlPlaneStopTimeout: 60 * time.Second,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(v1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(clusterv1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(ipamv1.AddToScheme(scheme.Scheme)).To(Succeed())

	//+kubebuilder:scaffold:scheme

	apiClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	// komega's Object/Get read through the live client, so assertions never observe a stale cache.
	komega.SetClient(apiClient)
	komega.SetContext(ctx)
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	Expect(testEnv.Stop()).To(Succeed())
})

// createNamespace creates a namespace with a generated name, isolating a spec from the objects of
// every other spec.
func createNamespace() string {
	namespaceObj := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "test-ns-"},
	}
	ExpectWithOffset(1, apiClient.Create(ctx, &namespaceObj)).To(Succeed())
	return namespaceObj.Name
}

// createObj creates an object on the API server.
func createObj(obj client.Object) {
	ExpectWithOffset(1, apiClient.Create(ctx, obj)).To(Succeed())
}

// newClaim builds an IPAddressClaim referencing the given pool.
func newClaim(name, namespace, poolKind, poolName string) ipamv1.IPAddressClaim {
	return ipamv1.IPAddressClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: ipamv1.IPAddressClaimSpec{
			PoolRef: ipamv1.IPPoolReference{
				APIGroup: "ipam.cluster.x-k8s.io",
				Kind:     poolKind,
				Name:     poolName,
			},
		},
	}
}

// haveReadyCondition matches an object whose Ready condition has the given status and reason.
func haveReadyCondition(status metav1.ConditionStatus, reason string) gomegatypes.GomegaMatcher {
	return HaveField("Status.Conditions", ContainElement(And(
		HaveField("Type", BeEquivalentTo(clusterv1.ReadyCondition)),
		HaveField("Status", BeEquivalentTo(status)),
		HaveField("Reason", BeEquivalentTo(reason)),
	)))
}

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/telekom/cluster-api-ipam-provider-infoblox/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These specs exercise the CRD schema itself, so they only need an API server. No reconciler is
// involved: every assertion is about what the API server accepts or rejects on create.
var _ = Describe("InfobloxIPPool CRD validation", func() {
	var namespace string

	BeforeEach(func() {
		namespace = createNamespace()
	})

	// createPool attempts to create a pool with the given subnets and reports the API server's
	// verdict. The pool is removed again if it was accepted.
	createPool := func(name string, subnets []v1alpha1.Subnet) error {
		pool := &v1alpha1.InfobloxIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: v1alpha1.InfobloxIPPoolSpec{
				InstanceRef: v1alpha1.InstanceReference{Name: "test-instance"},
				Subnets:     subnets,
			},
		}
		defer func() {
			ExpectWithOffset(1, client.IgnoreNotFound(apiClient.Delete(ctx, pool))).To(Succeed())
		}()
		return apiClient.Create(ctx, pool)
	}

	Context("CIDR validation", func() {
		It("should accept a valid IPv4 CIDR", func() {
			Expect(createPool("valid-v4-cidr", []v1alpha1.Subnet{{CIDR: "10.0.0.0/24", Gateway: "10.0.0.1"}})).To(Succeed())
		})

		It("should accept a valid IPv6 CIDR", func() {
			Expect(createPool("valid-v6-cidr", []v1alpha1.Subnet{{CIDR: "2001:db8::/64", Gateway: "2001:db8::1"}})).To(Succeed())
		})

		It("should reject a non-CIDR string", func() {
			err := createPool("invalid-cidr", []v1alpha1.Subnet{{CIDR: "not-a-cidr"}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.subnets[0].cidr"))
			Expect(err.Error()).To(ContainSubstring("Invalid value"))
		})

		It("should reject a CIDR without prefix", func() {
			err := createPool("no-prefix-cidr", []v1alpha1.Subnet{{CIDR: "10.0.0.0"}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.subnets[0].cidr"))
			Expect(err.Error()).To(ContainSubstring("Invalid value"))
		})

		It("should reject an empty CIDR", func() {
			err := createPool("empty-cidr", []v1alpha1.Subnet{{CIDR: ""}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.subnets"))
		})
	})

	Context("Gateway validation", func() {
		It("should accept a valid IPv4 gateway", func() {
			Expect(createPool("valid-v4-gw", []v1alpha1.Subnet{{CIDR: "10.0.0.0/24", Gateway: "10.0.0.1"}})).To(Succeed())
		})

		It("should accept a valid IPv6 gateway", func() {
			Expect(createPool("valid-v6-gw", []v1alpha1.Subnet{{CIDR: "2001:db8::/64", Gateway: "2001:db8::1"}})).To(Succeed())
		})

		It("should accept an empty gateway", func() {
			Expect(createPool("empty-gw", []v1alpha1.Subnet{{CIDR: "10.0.0.0/24", Gateway: ""}})).To(Succeed())
		})

		It("should reject a non-IP gateway", func() {
			err := createPool("invalid-gw", []v1alpha1.Subnet{{CIDR: "10.0.0.0/24", Gateway: "not-an-ip"}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.subnets[0].gateway"))
			Expect(err.Error()).To(ContainSubstring("Invalid value"))
		})

		It("should reject a gateway with CIDR notation", func() {
			err := createPool("cidr-gw", []v1alpha1.Subnet{{CIDR: "10.0.0.0/24", Gateway: "10.0.0.1/24"}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.subnets[0].gateway"))
			Expect(err.Error()).To(ContainSubstring("Invalid value"))
		})
	})

	Context("Subnet uniqueness (listType=map)", func() {
		It("should accept multiple subnets with different CIDRs", func() {
			Expect(createPool("unique-subnets", []v1alpha1.Subnet{
				{CIDR: "10.0.0.0/24", Gateway: "10.0.0.1"},
				{CIDR: "10.0.1.0/24", Gateway: "10.0.1.1"},
			})).To(Succeed())
		})

		It("should reject duplicate subnet CIDRs", func() {
			err := createPool("dup-subnets", []v1alpha1.Subnet{
				{CIDR: "10.0.0.0/24", Gateway: "10.0.0.1"},
				{CIDR: "10.0.0.0/24", Gateway: "10.0.0.2"},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Duplicate"))
		})
	})
})

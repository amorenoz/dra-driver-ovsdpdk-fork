/*
 * Copyright 2026 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package claimstore_test

import (
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"

	"github.com/k8snetworkplumbingwg/dra-driver-ovsdpdk/pkg/claimstore"
	dratypes "github.com/k8snetworkplumbingwg/dra-driver-ovsdpdk/pkg/types"
)

func TestClaimStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ClaimStore Suite")
}

var _ = Describe("PreparedClaimStore", func() {
	var cs claimstore.PreparedClaimStore

	BeforeEach(func() {
		var err error
		cs, err = claimstore.New()
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Get", func() {
		It("should return nil slice for an unknown claim UID", func() {
			got, err := cs.Get("unknown-uid")
			Expect(got).To(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return the stored PreparedDevice for known claim UID", func() {
			uid := k8stypes.UID("uid-1")
			pd := makePDs(uid, "claim-1")
			Expect(cs.Set(uid, pd)).To(Succeed())

			got, err := cs.Get(uid)
			Expect(err).ToNot(HaveOccurred())
			Expect(got).To(Equal(pd))
		})
	})

	Describe("Set", func() {
		It("should overwrite an existing entry", func() {
			uid := k8stypes.UID("uid-3")
			pd1 := makePDs(uid, "first")
			pd2 := makePDs(uid, "second")

			Expect(cs.Set(uid, pd1)).To(Succeed())
			Expect(cs.Set(uid, pd2)).To(Succeed())

			got, err := cs.Get(uid)
			Expect(err).ToNot(HaveOccurred())
			Expect(got[0].ClaimNamespacedName.Name).To(Equal("second"))
		})

		It("should store independent entries for different UIDs", func() {
			uid1 := k8stypes.UID("uid-a")
			uid2 := k8stypes.UID("uid-b")
			pd1 := makePDs(uid1, "claim-a")
			pd2 := makePDs(uid2, "claim-b")

			Expect(cs.Set(uid1, pd1)).To(Succeed())
			Expect(cs.Set(uid2, pd2)).To(Succeed())

			got1, _ := cs.Get(uid1)
			got2, _ := cs.Get(uid2)
			Expect(got1).To(Equal(pd1))
			Expect(got2).To(Equal(pd2))
		})
	})

	Describe("Delete", func() {
		It("should delete the entry", func() {
			uid := k8stypes.UID("uid-4")
			pd := makePDs(uid, "to-delete")
			Expect(cs.Set(uid, pd)).To(Succeed())
			Expect(cs.Delete(uid)).To(Succeed())

			got, err := cs.Get(uid)
			Expect(err).ToNot(HaveOccurred())
			Expect(got).To(BeNil())
		})
	})

	Describe("thread safety", func() {
		It("should handle concurrent Set and Get without data races", func() {
			const goroutines = 50
			var wg sync.WaitGroup
			wg.Add(goroutines * 2)

			for i := range goroutines {
				uid := k8stypes.UID(k8stypes.UID("uid-concurrent-" + string(rune('A'+i))))
				pd := makePDs(uid, "claim-concurrent")

				go func() {
					defer wg.Done()
					_ = cs.Set(uid, pd)
				}()
				go func() {
					defer wg.Done()
					_, _ = cs.Get(uid)
				}()
			}
			wg.Wait()
		})

		It("should handle concurrent Set and Delete without data races", func() {
			const goroutines = 50
			var wg sync.WaitGroup
			wg.Add(goroutines * 2)

			for i := range goroutines {
				uid := k8stypes.UID("uid-del-" + string(rune('A'+i)))
				pd := makePDs(uid, "claim-del")
				Expect(cs.Set(uid, pd)).To(Succeed())

				go func() {
					defer wg.Done()
					_ = cs.Set(uid, pd)
				}()
				go func() {
					defer wg.Done()
					_ = cs.Delete(uid)
				}()
			}
			wg.Wait()
		})
	})
})

// makePDs builds a slice with a single minimal PreparedDevice for testing the claim store.
func makePDs(uid k8stypes.UID, name string) []*dratypes.PreparedDevice {
	return []*dratypes.PreparedDevice{
		{
			ClaimNamespacedName: kubeletplugin.NamespacedObject{
				NamespacedName: k8stypes.NamespacedName{
					Name:      name,
					Namespace: "default",
				},
				UID: uid,
			},
		},
	}
}

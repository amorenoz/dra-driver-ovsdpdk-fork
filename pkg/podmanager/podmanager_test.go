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

package podmanager_test

import (
	"errors"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"

	"github.com/k8snetworkplumbingwg/dra-driver-ovsdpdk/pkg/podmanager"
	"github.com/k8snetworkplumbingwg/dra-driver-ovsdpdk/pkg/podmanager/mocks"
	dratypes "github.com/k8snetworkplumbingwg/dra-driver-ovsdpdk/pkg/types"
)

func TestPodManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PodManager Suite")
}

var _ = Describe("PodManager", func() {
	var pm *podmanager.PodManager
	var cp *mocks.MockCheckpoint

	BeforeEach(func() {
		var err error
		cp = mocks.NewMockCheckpoint(GinkgoT())
		cp.EXPECT().Load().Return(map[k8stypes.UID][]*dratypes.PreparedDevice{}, nil)

		pm, err = podmanager.New(cp)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Get", func() {
		It("should return false for an unknown claim UID", func() {
			_, found := pm.Get("unknown-uid")
			Expect(found).To(BeFalse())
		})

		It("should return the stored PreparedDevice and true for a known claim UID", func() {
			uid := k8stypes.UID("uid-1")
			pd := makePDs(uid, "claim-1")

			cp.EXPECT().Store(uid, pd).Return(nil)
			Expect(pm.Set(uid, pd)).To(Succeed())

			got, found := pm.Get(uid)
			Expect(found).To(BeTrue())
			Expect(got).To(Equal(pd))
		})

		It("should not remove the entry on Get", func() {
			uid := k8stypes.UID("uid-2")
			pd := makePDs(uid, "claim-2")

			cp.EXPECT().Store(uid, pd).Return(nil)
			Expect(pm.Set(uid, pd)).To(Succeed())

			pm.Get(uid)
			_, found := pm.Get(uid)
			Expect(found).To(BeTrue())
		})
	})

	Describe("Set", func() {
		It("should overwrite an existing entry", func() {
			uid := k8stypes.UID("uid-3")
			pd1 := makePDs(uid, "first")
			pd2 := makePDs(uid, "second")

			cp.EXPECT().Store(uid, pd1).Return(nil)
			Expect(pm.Set(uid, pd1)).To(Succeed())

			cp.EXPECT().Store(uid, pd2).Return(nil)
			Expect(pm.Set(uid, pd2)).To(Succeed())

			got, found := pm.Get(uid)
			Expect(found).To(BeTrue())
			Expect(got[0].ClaimNamespacedName.Name).To(Equal("second"))
		})

		It("should store independent entries for different UIDs", func() {
			uid1 := k8stypes.UID("uid-a")
			uid2 := k8stypes.UID("uid-b")
			pd1 := makePDs(uid1, "claim-a")
			pd2 := makePDs(uid2, "claim-b")

			cp.EXPECT().Store(uid1, pd1).Return(nil)
			Expect(pm.Set(uid1, pd1)).To(Succeed())

			cp.EXPECT().Store(uid2, pd2).Return(nil)
			Expect(pm.Set(uid2, pd2)).To(Succeed())

			got1, _ := pm.Get(uid1)
			got2, _ := pm.Get(uid2)
			Expect(got1).To(Equal(pd1))
			Expect(got2).To(Equal(pd2))
		})

		It("should not update the in-memory cache when checkpoint Store fails", func() {
			uid := k8stypes.UID("uid-store-fail")
			pd := makePDs(uid, "claim-store-fail")

			storeErr := errors.New("disk full")
			cp.EXPECT().Store(uid, pd).Return(storeErr)
			Expect(pm.Set(uid, pd)).To(MatchError(storeErr))

			// The in-memory cache must not contain the entry.
			_, found := pm.Get(uid)
			Expect(found).To(BeFalse())
		})
	})

	Describe("Delete", func() {
		It("should return nil for an unknown claim UID", func() {
			Expect(pm.Delete("nonexistent")).To(BeNil())
		})

		It("should return the PreparedDevice and remove it from the cache", func() {
			uid := k8stypes.UID("uid-4")
			pd := makePDs(uid, "to-delete")

			cp.EXPECT().Store(uid, pd).Return(nil)
			Expect(pm.Set(uid, pd)).To(Succeed())

			cp.EXPECT().Delete(uid).Return(nil)
			got := pm.Delete(uid)
			Expect(got).To(Equal(pd))

			_, found := pm.Get(uid)
			Expect(found).To(BeFalse())
		})

		It("should return nil on a second delete of the same UID", func() {
			uid := k8stypes.UID("uid-5")
			pd := makePDs(uid, "claim-5")

			cp.EXPECT().Store(uid, pd).Return(nil)
			Expect(pm.Set(uid, pd)).To(Succeed())

			cp.EXPECT().Delete(uid).Return(nil)
			pm.Delete(uid)
			Expect(pm.Delete(uid)).To(BeNil())
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
					cp.EXPECT().Store(uid, pd).Return(nil)
					_ = pm.Set(uid, pd)
				}()
				go func() {
					defer wg.Done()
					pm.Get(uid)
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

				cp.EXPECT().Store(uid, pd).Return(nil)
				Expect(pm.Set(uid, pd)).To(Succeed())

				go func() {
					defer wg.Done()
					cp.EXPECT().Store(uid, pd).Return(nil)
					_ = pm.Set(uid, pd)
				}()
				go func() {
					defer wg.Done()
					cp.EXPECT().Delete(uid).Return(nil)
					pm.Delete(uid)
				}()
			}
			wg.Wait()
		})
	})
})

// makePDs builds a slice with a single minimal PreparedDevice for testing the pod manager cache.
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

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

// Package claimstore persists prepared resource-claim state between Prepare
// and Unprepare calls in the driver layer.
package claimstore

import (
	"sync"

	k8stypes "k8s.io/apimachinery/pkg/types"

	dratypes "github.com/k8snetworkplumbingwg/dra-driver-ovsdpdk/pkg/types"
)

// preparedClaimStore is a thread-safe in-memory store of PreparedDevice
// records keyed by claim UID.
type preparedClaimStore struct {
	mu         sync.RWMutex
	byClaimUID map[k8stypes.UID][]*dratypes.PreparedDevice
}

// New creates a new PreparedClaimStore.
func New() (PreparedClaimStore, error) {
	return &preparedClaimStore{
		byClaimUID: make(map[k8stypes.UID][]*dratypes.PreparedDevice),
	}, nil
}

// Get returns the PreparedDevices for the given claim UID.
// Returns nil slice if not found.
func (s *preparedClaimStore) Get(claimUID k8stypes.UID) ([]*dratypes.PreparedDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byClaimUID[claimUID], nil
}

// Set stores the PreparedDevices for the given claim UID.
func (s *preparedClaimStore) Set(claimUID k8stypes.UID, sc []*dratypes.PreparedDevice) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byClaimUID[claimUID] = sc
	return nil
}

// Delete removes the PreparedDevices for the given claim UID.
func (s *preparedClaimStore) Delete(claimUID k8stypes.UID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byClaimUID, claimUID)
	return nil
}

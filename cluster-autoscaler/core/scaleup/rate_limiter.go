/*
Copyright 2016 The Kubernetes Authors.

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

package scaleup

import (
	"sync"
	"time"
)

// ScaleUpRateLimiter is a struct that handles the rate at which new nodes can be added.
// It's a solution for the problem where the API server seems to be unresponsive when the cluster scales up too quickly.
// This issues renders the cluster to be unresponsive to kubectl and cortex commands.
// It also might be related to cortex cluster down not cleaning up all of the resources.
type ScaleUpRateLimiter struct {
	// targeted number of nodes per min
	MaxNumberOfNodesPerMin int
	// burst number of nodes per min
	BurstMaxNumberOfNodesPerMin int
	// node slots that haven't been used in the previous iteration
	UnusedNodeSlots int
	// last reserve time
	LastReserve time.Time
	mu          sync.Mutex
}

// AcquireNodes is a method of ScaleUpRateLimiter that decides the number of new nodes that can be added
// based on the constraints and the number of nodes requested. It returns a boolean indicating if nodes
// can be added and the number of nodes that can be added.
func (t *ScaleUpRateLimiter) AcquireNodes(newNodes int) (bool, int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	allowedNumNodesToAdd := int(now.Sub(t.LastReserve).Minutes())*t.MaxNumberOfNodesPerMin + t.UnusedNodeSlots
	if allowedNumNodesToAdd > t.BurstMaxNumberOfNodesPerMin {
		allowedNumNodesToAdd = t.BurstMaxNumberOfNodesPerMin
	}

	if allowedNumNodesToAdd <= 0 {
		// no quota, can not scale up
		return false, 0
	}

	t.LastReserve = now
	if newNodes > allowedNumNodesToAdd {
		t.UnusedNodeSlots = 0
		return true, allowedNumNodesToAdd
	}
	t.UnusedNodeSlots = allowedNumNodesToAdd - newNodes

	return true, newNodes
}

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

// ScaleUpRateLimiter represents a rate limiter for scaling up nodes.
// It manages the number of nodes that can be added per minute based on a
// maximum limit and a burst limit.
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

// AcquireNodes tries to reserve a number of nodes for scale up.
// The function returns a boolean value indicating whether nodes can be scaled up,
// and an integer value representing the number of nodes that can be added.
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

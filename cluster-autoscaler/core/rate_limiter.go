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

package core

import (
	"sync"
	"time"
)

type ScaleUpRateLimiter struct {
	// targeted number of nodes per min
	maxNumberOfNodesPerMin int
	// burst number of nodes per min
	burstMaxNumberOfNodesPerMin int
	// node slots that haven't been used in the previous iteration
	unusedNodeSlots int
	// last reserve time
	lastReserve time.Time
	mu          sync.Mutex
}

func (t *ScaleUpRateLimiter) AcquireNodes(newNodes int) (bool, int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	allowedNumNodesToAdd := int(now.Sub(t.lastReserve).Minutes())*t.maxNumberOfNodesPerMin + t.unusedNodeSlots
	if allowedNumNodesToAdd > t.burstMaxNumberOfNodesPerMin {
		allowedNumNodesToAdd = t.burstMaxNumberOfNodesPerMin
	}

	if allowedNumNodesToAdd <= 0 {
		// no quota, can not scale up
		return false, 0
	}

	t.lastReserve = now
	if newNodes > allowedNumNodesToAdd {
		t.unusedNodeSlots = 0
		return true, allowedNumNodesToAdd
	}
	t.unusedNodeSlots = allowedNumNodesToAdd - newNodes

	return true, newNodes
}

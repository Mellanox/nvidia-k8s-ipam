/*
 Copyright 2023, NVIDIA CORPORATION & AFFILIATES
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

package selector

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// New creates a new selector instance
func New() *Selector {
	return &Selector{}
}

// Selector holds labels selector
type Selector struct {
	lock     sync.RWMutex
	selector map[string]string
}

// Match check if selector match the node
func (s *Selector) Match(node *corev1.Node) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if len(s.selector) == 0 {
		return true
	}
	return labels.Set(s.selector).AsSelector().Matches(labels.Set(node.GetLabels()))
}

// Update label selector
func (s *Selector) Update(newSelector map[string]string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.selector = newSelector
}

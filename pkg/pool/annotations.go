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

package pool

import (
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
)

// SetIPBlockAnnotation serialize IP pools settings for the node and add this info as annotation
func SetIPBlockAnnotation(node *v1.Node, pools map[string]*IPPool) error {
	annotations := node.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	data, err := json.Marshal(pools)
	if err != nil {
		return fmt.Errorf("failed to serialize pools config: %v", err)
	}
	annotations[ipBlocksAnnotation] = string(data)
	node.SetAnnotations(annotations)
	return nil
}

// IPBlockAnnotationExists returns true if ip-block annotation exist
func IPBlockAnnotationExists(node *v1.Node) bool {
	_, exist := node.GetAnnotations()[ipBlocksAnnotation]
	return exist
}

// RemoveIPBlockAnnotation removes annotation with ip-block from the node object
func RemoveIPBlockAnnotation(node *v1.Node) {
	annotations := node.GetAnnotations()
	delete(annotations, ipBlocksAnnotation)
	node.SetAnnotations(annotations)
}

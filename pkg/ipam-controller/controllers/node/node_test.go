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

package controllers

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func testNode(labels, annotations map[string]string, ready corev1.ConditionStatus) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: labels, Annotations: annotations, ResourceVersion: "1"},
		Status:     corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: ready}}},
	}
}

func TestNodeEventPredicate(t *testing.T) {
	p := nodeEventPredicate()
	base := testNode(map[string]string{"role": "gpu"}, map[string]string{"a": "1"}, corev1.ConditionTrue)

	if !p.Create(event.CreateEvent{Object: base}) {
		t.Error("create must pass")
	}
	if !p.Delete(event.DeleteEvent{Object: base}) {
		t.Error("delete must pass")
	}
	if !p.Generic(event.GenericEvent{Object: base}) {
		t.Error("generic must pass")
	}

	cases := []struct {
		name string
		new  *corev1.Node
		want bool
	}{
		{"status only", testNode(map[string]string{"role": "gpu"}, map[string]string{"a": "1"}, corev1.ConditionFalse), false},
		{"annotation only", testNode(map[string]string{"role": "gpu"}, map[string]string{"a": "2"}, corev1.ConditionTrue), false},
		{"label value changed", testNode(map[string]string{"role": "cpu"}, map[string]string{"a": "1"}, corev1.ConditionTrue), true},
		{"label added", testNode(map[string]string{"role": "gpu", "x": "y"}, nil, corev1.ConditionTrue), true},
		{"label removed", testNode(map[string]string{}, nil, corev1.ConditionTrue), true},
	}
	for _, c := range cases {
		c.new.ResourceVersion = "2"
		if got := p.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: c.new}); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
	if !p.Update(event.UpdateEvent{ObjectOld: nil, ObjectNew: base}) {
		t.Error("update with missing old object must pass")
	}
}

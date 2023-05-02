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
	"context"
	"encoding/json"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/allocator"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/config"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/selector"
)

// ConfigMapReconciler reconciles ConfigMap objects
type ConfigMapReconciler struct {
	Allocator          *allocator.Allocator
	Selector           *selector.Selector
	ConfigMapName      string
	ConfigMapNamespace string

	ConfigEventCh chan event.GenericEvent
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile contains logic to sync Node objects
func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLog := log.FromContext(ctx)
	if req.Name != r.ConfigMapName || req.Namespace != r.ConfigMapNamespace {
		// this should never happen because of the watcher configuration of the manager from controller-runtime pkg
		return ctrl.Result{}, nil
	}

	cfg := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, req.NamespacedName, cfg)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			reqLog.Info("ConfigMap not found, wait for creation")
			return ctrl.Result{}, nil
		}
		reqLog.Error(err, "failed to get ConfigMap object from the cache")
		return ctrl.Result{}, err
	}

	confData, exist := cfg.Data[config.ConfigMapKey]
	if !exist {
		reqLog.Error(nil, fmt.Sprintf("invalid configuration: ConfigMap %s doesn't contain %s key",
			r.ConfigMapNamespace, config.ConfigMapKey))
		return ctrl.Result{}, nil
	}

	controllerConfig := &config.Config{}
	if err := json.Unmarshal([]byte(confData), controllerConfig); err != nil {
		reqLog.Error(err, fmt.Sprintf("invalid configuration: ConfigMap %s contains invalid JSON",
			config.ConfigMapKey))
		return ctrl.Result{}, nil
	}

	if err := controllerConfig.Validate(); err != nil {
		reqLog.Error(err, fmt.Sprintf("invalid configuration: ConfigMap %s contains invalid config",
			config.ConfigMapKey))
		return ctrl.Result{}, nil
	}

	r.Selector.Update(controllerConfig.NodeSelector)

	allocatorPools := make([]allocator.AllocationConfig, 0, len(controllerConfig.Pools))
	for pName, p := range controllerConfig.Pools {
		// already validated by Validate function
		_, subnet, _ := net.ParseCIDR(p.Subnet)
		gateway := net.ParseIP(p.Gateway)
		allocatorPools = append(allocatorPools, allocator.AllocationConfig{
			PoolName:         pName,
			Subnet:           subnet,
			Gateway:          gateway,
			PerNodeBlockSize: p.PerNodeBlockSize,
		})
	}

	if r.Allocator.IsConfigured() {
		r.Allocator.Configure(ctx, allocatorPools)
	} else {
		nodeList := &corev1.NodeList{}
		if err := r.Client.List(ctx, nodeList); err != nil {
			return ctrl.Result{}, err
		}
		r.Allocator.ConfigureAndLoadAllocations(ctx, allocatorPools, nodeList.Items)
	}

	// config updated, trigger sync for all nodes
	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList); err != nil {
		return ctrl.Result{}, err
	}
	for _, n := range nodeList.Items {
		r.ConfigEventCh <- event.GenericEvent{
			Object: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: n.Name},
			}}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Complete(r)
}

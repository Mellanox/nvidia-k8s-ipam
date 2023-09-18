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

package migrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/config"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

const (
	// EnvDisableMigration contains the name of the environment variable which can be used
	// to disable migration
	EnvDisableMigration = "MIGRATOR_DISABLE_MIGRATION"
	// EnvConfigMapName contains the name of the environment variable which can be used
	// to specify the ConfigMap name containing the configuration to migrate from
	EnvConfigMapName = "CONFIGMAP_NAME"
	// EnvConfigMapNamespace contains the name of the environment variable which can be used
	// to specify the namespace of the ConfigMap containing the configuration to migrate from
	EnvConfigMapNamespace = "CONFIGMAP_NAMESPACE"
	// DefaultConfigMapName is the default ConfigMap name used to read the configuration to
	// migrate from
	DefaultConfigMapName = "nvidia-k8s-ipam-config"
)

// Migrator migrate from CM config to IPPool CR
type Migrator struct {
	K8sClient        client.Client
	IPPoolsNamespace string
	MigrationCh      chan struct{}
	LeaderElection   bool
	Logger           logr.Logger
}

// Implements manager.Runnable
func (m *Migrator) Start(ctx context.Context) error {
	err := Migrate(ctx, m.Logger, m.K8sClient, m.IPPoolsNamespace)
	if err != nil {
		m.Logger.Error(err, fmt.Sprintf("failed to migrate NV-IPAM config from ConfigMap, "+
			"set %s env variable to disable migration", EnvDisableMigration))
		return err
	}
	close(m.MigrationCh)
	return nil
}

// Implements manager.NeedLeaderElection
func (m *Migrator) NeedLeaderElection() bool {
	return m.LeaderElection
}

// Migrate reads the ConfigMap with the IPAM configuration, reads the allocations
// from the Nodes annotation, create IPPool CRs and delete the ConfigMap and annotations
func Migrate(ctx context.Context, logger logr.Logger, c client.Client, poolNamespace string) error {
	if os.Getenv(EnvDisableMigration) != "" {
		logger.Info(fmt.Sprintf("%s set, skip controller migration", EnvDisableMigration))
		return nil
	}

	cmName := DefaultConfigMapName
	if os.Getenv(EnvConfigMapName) != "" {
		cmName = os.Getenv(EnvConfigMapName)
	}
	cmNamespace := poolNamespace
	if os.Getenv(EnvConfigMapNamespace) != "" {
		cmNamespace = os.Getenv(EnvConfigMapNamespace)
	}

	cfg := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Name:      cmName,
		Namespace: cmNamespace,
	}
	err := c.Get(ctx, key, cfg)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			logger.Info("ConfigMap not found, skipping migration")
			return nil
		}
		logger.Error(err, "failed to read ConfigMap object")
		return err
	}

	confData, exist := cfg.Data[config.ConfigMapKey]
	if !exist {
		err = fmt.Errorf("invalid configuration: ConfigMap %s doesn't contain %s key",
			key, config.ConfigMapKey)
		logger.Error(err, "Invalid config, no data")
		return err
	}
	controllerConfig := &config.Config{}
	if err := json.Unmarshal([]byte(confData), controllerConfig); err != nil {
		logger.Error(err, fmt.Sprintf("invalid configuration: ConfigMap %s contains invalid JSON",
			config.ConfigMapKey))
		return err
	}
	if err := controllerConfig.Validate(); err != nil {
		logger.Error(err, fmt.Sprintf("invalid configuration: ConfigMap %s contains invalid config",
			config.ConfigMapKey))
		return err
	}

	pools := buildIPPools(controllerConfig, poolNamespace)

	for name, p := range pools {
		err = c.Create(ctx, p)
		logger.Info(fmt.Sprintf("Creating IPPool: %v", p))
		if apiErrors.IsAlreadyExists(err) {
			existingPool := &ipamv1alpha1.IPPool{}
			err = c.Get(ctx, client.ObjectKeyFromObject(p), existingPool)
			if err != nil {
				logger.Info("fail to get existing pool", "pool name", name)
				return err
			}
			if !reflect.DeepEqual(existingPool.Spec, p.Spec) {
				logger.Info("existing pool has different spec than config map setting", "pool name", name)
				return fmt.Errorf("existing pool has different spec than config map setting")
			}
		} else if err != nil {
			logger.Info("fail to create pool", "pool name", name)
			return err
		}
	}

	err = updateAllocations(ctx, c, logger, pools, poolNamespace)
	if err != nil {
		return err
	}

	err = c.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: cmNamespace, Name: cmName}})
	if err != nil {
		logger.Info("fail to delete nv-ipam config map")
		return err
	}

	return nil
}

func updateAllocations(ctx context.Context, c client.Client,
	logger logr.Logger, pools map[string]*ipamv1alpha1.IPPool, poolNamespace string) error {
	nodeList := &corev1.NodeList{}
	if err := c.List(ctx, nodeList); err != nil {
		logger.Error(err, "failed to list nodes")
		return err
	}
	nodesToClearAnnotation := sets.New[string]()
	for poolName := range pools {
		p := &ipamv1alpha1.IPPool{}
		key := types.NamespacedName{
			Name:      poolName,
			Namespace: poolNamespace,
		}
		err := c.Get(ctx, key, p)
		if err != nil {
			logger.Info("fail getting IPPool", "reason", err.Error())
			return err
		}
		if len(p.Status.Allocations) > 0 {
			logger.Info("skipping migration for IPPool, already has allocation", "ippool", poolName)
			continue
		}
		allocs := make([]ipamv1alpha1.Allocation, 0)
		for i := range nodeList.Items {
			node := nodeList.Items[i]
			nodeLog := logger.WithValues("node", node.Name)
			poolCfg, err := pool.NewConfigReader(&node)
			if err != nil {
				nodeLog.Info("skip loading data from the node", "reason", err.Error())
				continue
			}
			nodesToClearAnnotation.Insert(node.Name)
			nodeIPPoolConfig := poolCfg.GetPoolByName(poolName)
			if nodeIPPoolConfig == nil {
				nodeLog.Info("skip loading data for pool from the node, pool not configured", "node", node.Name, "pool", poolName)
				continue
			}
			alloc := ipamv1alpha1.Allocation{
				NodeName: node.Name,
				StartIP:  nodeIPPoolConfig.StartIP,
				EndIP:    nodeIPPoolConfig.EndIP,
			}
			allocs = append(allocs, alloc)
		}
		if len(allocs) != 0 {
			p.Status.Allocations = allocs
			logger.Info(fmt.Sprintf("Updating IPPool status: %v", p))
			err = c.Status().Update(ctx, p)
			if err != nil {
				logger.Info("fail to update pool allocation from node", "reason", err.Error())
				return err
			}
		}
	}

	for _, nodeName := range sets.List(nodesToClearAnnotation) {
		logger.Info("clear IPBlocksAnnotation from node", "name", nodeName)
		fmtKey := strings.ReplaceAll(pool.IPBlocksAnnotation, "/", "~1")
		patch := []byte(fmt.Sprintf("[{\"op\": \"remove\", \"path\": \"/metadata/annotations/%s\"}]", fmtKey))
		err := c.Patch(ctx, &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
		}, client.RawPatch(types.JSONPatchType, patch))
		if err != nil {
			logger.Info("fail to remove IPBlocksAnnotation from node", "name", nodeName, "reason", err.Error())
			return err
		}
	}
	return nil
}

func buildIPPools(controllerConfig *config.Config, poolNamespace string) map[string]*ipamv1alpha1.IPPool {
	var nodeSelector *corev1.NodeSelector
	if len(controllerConfig.NodeSelector) > 0 {
		nodeSelector = &corev1.NodeSelector{}
		selectorsItems := make([]corev1.NodeSelectorTerm, 0, len(controllerConfig.NodeSelector))
		for k, v := range controllerConfig.NodeSelector {
			selector := corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      k,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{v},
					},
				},
				MatchFields: nil,
			}
			selectorsItems = append(selectorsItems, selector)
		}
		nodeSelector.NodeSelectorTerms = selectorsItems
	}
	pools := make(map[string]*ipamv1alpha1.IPPool)
	for pName, p := range controllerConfig.Pools {
		// already validated by Validate function
		_, subnet, _ := net.ParseCIDR(p.Subnet)
		pools[pName] = &ipamv1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pName,
				Namespace: poolNamespace,
			},
			Spec: ipamv1alpha1.IPPoolSpec{
				Subnet:           subnet.String(),
				Gateway:          p.Gateway,
				PerNodeBlockSize: p.PerNodeBlockSize,
				NodeSelector:     nodeSelector,
			},
		}
	}
	return pools
}

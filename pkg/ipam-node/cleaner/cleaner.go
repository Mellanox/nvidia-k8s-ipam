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

package cleaner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	apiTypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

// Cleaner is the interface of the cleaner package.
// The cleaner periodically scan the store and check for allocations which doesn't have
// related Pod in the k8s API. If allocation has no Pod for more than X checks, then the cleaner
// will release the allocation. Also, the cleaner will remove pool entries in the store if the pool has no
// allocation and pool configuration is unavailable in the Kubernetes API.
type Cleaner interface {
	// Start starts the cleaner loop.
	// The cleaner loop discovers stale allocations and clean up them.
	Start(ctx context.Context)
}

// New creates and initialize new cleaner instance
// "checkInterval" defines delay between checks for stale allocations.
// "checkCountBeforeRelease: defines how many check to do before remove the allocation
func New(cachedClient client.Client, directClient client.Client, store storePkg.Store, poolConfReader pool.ConfigReader,
	checkInterval time.Duration,
	checkCountBeforeRelease int) Cleaner {
	return &cleaner{
		cachedClient:            cachedClient,
		directClient:            directClient,
		store:                   store,
		poolConfReader:          poolConfReader,
		checkInterval:           checkInterval,
		checkCountBeforeRelease: checkCountBeforeRelease,
		staleAllocations:        make(map[string]int),
	}
}

type cleaner struct {
	cachedClient            client.Client
	directClient            client.Client
	store                   storePkg.Store
	poolConfReader          pool.ConfigReader
	checkInterval           time.Duration
	checkCountBeforeRelease int
	// key is <pool_name>|<container_id>|<interface_name>, value is count of failed checks
	staleAllocations map[string]int
}

func (c *cleaner) Start(ctx context.Context) {
	logger := logr.FromContextOrDiscard(ctx)
	for {
		loopLogger := logger.WithValues("checkID", uuid.NewString())
		loopLogger.Info("check for stale IPs")
		ctx = logr.NewContext(ctx, loopLogger)
		if err := c.loop(ctx); err != nil {
			loopLogger.Error(err, "check failed")
		}
		select {
		case <-ctx.Done():
			logger.Info("shutdown cleaner")
			return
		case <-time.After(c.checkInterval):
		}
	}
}

func (c *cleaner) loop(ctx context.Context) error {
	logger := logr.FromContextOrDiscard(ctx)
	session, err := c.store.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed to open store: %v", err)
	}
	allReservations := map[string]struct{}{}
	emptyPools := []string{}
	for _, poolKey := range session.ListPools() {
		poolReservations := session.ListReservations(poolKey)
		if len(poolReservations) == 0 {
			emptyPools = append(emptyPools, poolKey)
			continue
		}
		for _, reservation := range poolReservations {
			resLogger := logger.WithValues("poolKey", poolKey,
				"container_id", reservation.ContainerID, "interface_name", reservation.InterfaceName)
			staleAllocKey := c.getStaleAllocKey(poolKey, reservation)
			allReservations[staleAllocKey] = struct{}{}
			if reservation.Metadata.PodName == "" || reservation.Metadata.PodNamespace == "" {
				resLogger.V(2).Info("reservation has no required metadata fields, skip")
				continue
			}
			// first check in the cache
			found, err := c.checkPod(ctx, c.cachedClient, reservation)
			if err != nil {
				session.Cancel()
				return fmt.Errorf("failed to read Pod info from the cache: %v", err)
			}
			if !found {
				resLogger.V(2).Info("cache check failed for the Pod, check in the API")
				// pod not found in the cache, try to query API directly to make sure
				// that we use latest state
				found, err = c.checkPod(ctx, c.directClient, reservation)
				if err != nil {
					session.Cancel()
					return fmt.Errorf("failed to read Pod info from the API: %v", err)
				}
			}
			if found {
				delete(c.staleAllocations, staleAllocKey)
			} else {
				c.staleAllocations[staleAllocKey]++
				resLogger.V(2).Info("pod not found, increase stale counter", "value", c.staleAllocations[staleAllocKey])
			}
		}
	}

	for k, count := range c.staleAllocations {
		// remove unknown reservations from c.staleAllocations
		if _, isKnownReservation := allReservations[k]; !isKnownReservation {
			delete(c.staleAllocations, k)
			continue
		}
		// release reservations which were marked as stale multiple times
		if count > c.checkCountBeforeRelease {
			keyFields := strings.SplitN(k, "|", 3)
			poolKey, containerID, ifName := keyFields[0], keyFields[1], keyFields[2]
			logger.Info("stale reservation released", "poolKey", poolKey,
				"container_id", containerID, "interface_name", ifName)
			session.ReleaseReservationByID(poolKey, containerID, ifName)
		}
	}
	// remove empty pools if they don't have configuration in the k8s API
	for _, emptyPool := range emptyPools {
		if p := c.poolConfReader.GetPoolByKey(emptyPool); p == nil {
			session.RemovePool(emptyPool)
		}
	}
	if err := session.Commit(); err != nil {
		return fmt.Errorf("failed to commit changes to the store: %v", err)
	}
	return nil
}

// return true if pod exist, error in case if an unknown error occurred
func (c *cleaner) checkPod(ctx context.Context, k8sClient client.Client, reservation types.Reservation) (bool, error) {
	pod := &corev1.Pod{}
	err := k8sClient.Get(ctx, apiTypes.NamespacedName{
		Namespace: reservation.Metadata.PodNamespace,
		Name:      reservation.Metadata.PodName,
	}, pod)
	if err != nil && !apiErrors.IsNotFound(err) {
		return false, fmt.Errorf("failed to read Pod info: %v", err)
	}
	if apiErrors.IsNotFound(err) ||
		(reservation.Metadata.PodUUID != "" && reservation.Metadata.PodUUID != string(pod.UID)) {
		// pod not found or it has different UUID (was recreated)
		return false, nil
	}
	// pod exist
	return true, nil
}

func (c *cleaner) getStaleAllocKey(poolName string, r types.Reservation) string {
	return fmt.Sprintf("%s|%s|%s", poolName, r.ContainerID, r.InterfaceName)
}

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
)

// Cleaner is the interface of the cleaner package.
// The cleaner periodically scan the store and check for allocations which doesn't have
// related Pod in the k8s API. If allocation has no Pod for more than X checks, then the cleaner
// will release the allocation.
type Cleaner interface {
	// Start starts the cleaner loop.
	// The cleaner loop discovers stale allocations and clean up them.
	Start(ctx context.Context)
}

// New creates and initialize new cleaner instance
// "checkInterval" defines delay between checks for stale allocations.
// "checkCountBeforeRelease: defines how many check to do before remove the allocation
func New(client client.Client, store storePkg.Store,
	checkInterval time.Duration,
	checkCountBeforeRelease int) Cleaner {
	return &cleaner{
		client:                  client,
		store:                   store,
		checkInterval:           checkInterval,
		checkCountBeforeRelease: checkCountBeforeRelease,
		staleAllocations:        make(map[string]int),
	}
}

type cleaner struct {
	client                  client.Client
	store                   storePkg.Store
	checkInterval           time.Duration
	checkCountBeforeRelease int
	// key is <pool_name>|<container_id>|<interface_name>, value is count of failed checks
	staleAllocations map[string]int
}

func (c *cleaner) Start(ctx context.Context) {
	logger := logr.FromContextOrDiscard(ctx).WithName("cleaner")
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
	store, err := c.store.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed to open store: %v", err)
	}
	allReservations := map[string]struct{}{}
	for _, poolName := range store.ListPools() {
		for _, reservation := range store.ListReservations(poolName) {
			resLogger := logger.WithValues("pool", poolName,
				"container_id", reservation.ContainerID, "interface_name", reservation.InterfaceName)
			key := c.getStaleAllocKey(poolName, reservation)
			allReservations[key] = struct{}{}
			if reservation.Metadata.PodName == "" || reservation.Metadata.PodNamespace == "" {
				resLogger.V(2).Info("reservation has no required metadata fields, skip")
				continue
			}
			pod := &corev1.Pod{}
			err := c.client.Get(ctx, apiTypes.NamespacedName{
				Namespace: reservation.Metadata.PodNamespace,
				Name:      reservation.Metadata.PodName,
			}, pod)
			if err != nil && !apiErrors.IsNotFound(err) {
				store.Cancel()
				return fmt.Errorf("failed to read Pod info from the cache: %v", err)
			}
			if apiErrors.IsNotFound(err) ||
				(reservation.Metadata.PodUUID != "" && reservation.Metadata.PodUUID != string(pod.UID)) {
				c.staleAllocations[key]++
				resLogger.V(2).Info("pod not found in the API, increase stale counter",
					"value", c.staleAllocations[key])
			} else {
				delete(c.staleAllocations, key)
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
			pool, containerID, ifName := keyFields[0], keyFields[1], keyFields[2]
			logger.Info("stale reservation released", "pool", pool,
				"container_id", containerID, "interface_name", ifName)
			store.ReleaseReservationByID(pool, containerID, ifName)
		}
	}
	if err := store.Commit(); err != nil {
		return fmt.Errorf("failed to commit changes to the store: %v", err)
	}
	return nil
}

func (c *cleaner) getStaleAllocKey(poolName string, r types.Reservation) string {
	return fmt.Sprintf("%s|%s|%s", poolName, r.ContainerID, r.InterfaceName)
}

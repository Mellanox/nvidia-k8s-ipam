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

package types

import (
	"encoding/json"
	"errors"
	"hash/fnv"
	"time"
)

var (
	ErrCorruptData = errors.New("data corrupted, checksum mismatch")
)

// Checksum is the data to be stored as checkpoint
type Checksum uint32

// Verify verifies that passed checksum is same as calculated checksum
func (cs Checksum) Verify(r *Root) error {
	if cs != NewChecksum(r) {
		return ErrCorruptData
	}
	return nil
}

// NewChecksum returns the Checksum of the object
func NewChecksum(r *Root) Checksum {
	return Checksum(getChecksum(r))
}

// Get returns calculated checksum for the Root object
func getChecksum(r *Root) uint32 {
	stripped := r.DeepCopy()
	stripped.Checksum = 0
	// Zero ReleasedAt: an older binary's Reservation type predates this field and drops
	// it on load, so including it here would make that binary compute a different
	// checksum than the one stored, breaking rollback (nvidia-k8s-ipam#301).
	for _, pool := range stripped.Pools {
		for key, entry := range pool.Entries {
			entry.ReleasedAt = time.Time{}
			pool.Entries[key] = entry
		}
	}

	h := fnv.New32a()
	data, err := json.Marshal(stripped)
	if err != nil {
		panic("failed to compute checksum for input data")
	}
	_, _ = h.Write(data)
	return h.Sum32()
}

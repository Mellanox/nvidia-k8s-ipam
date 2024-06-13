/*
 Copyright 2024, NVIDIA CORPORATION & AFFILIATES
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

package common

// GetPoolKey builds uniq key for pool from poolName and poolType
func GetPoolKey(poolName, poolType string) string {
	if poolType == "" || poolType == PoolTypeIPPool {
		// to avoid migration of the store, and to support downgrade
		return poolName
	}
	return poolType + "/" + poolName
}

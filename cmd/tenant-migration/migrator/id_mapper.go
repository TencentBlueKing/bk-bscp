/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package migrator

import (
	"sync"
)

// IDMapper stores the mapping from source ID to target ID for each table.
// This is used for foreign key conversion during migration and Vault path updates.
type IDMapper struct {
	mu       sync.RWMutex
	mappings map[string]map[uint32]uint32 // table -> sourceID -> targetID
}

// NewIDMapper creates a new IDMapper instance
func NewIDMapper() *IDMapper {
	return &IDMapper{
		mappings: make(map[string]map[uint32]uint32),
	}
}

// Set stores a mapping from source ID to target ID for a specific table
func (m *IDMapper) Set(table string, sourceID, targetID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mappings[table] == nil {
		m.mappings[table] = make(map[uint32]uint32)
	}
	m.mappings[table][sourceID] = targetID
}

// Get retrieves the target ID for a given source ID in a specific table.
// Returns 0 if the mapping doesn't exist.
func (m *IDMapper) Get(table string, sourceID uint32) uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.mappings[table] == nil {
		return 0
	}
	return m.mappings[table][sourceID]
}

// GetOrDefault retrieves the target ID for a given source ID.
// If the mapping doesn't exist, returns the source ID as default.
func (m *IDMapper) GetOrDefault(table string, sourceID uint32) uint32 {
	if targetID := m.Get(table, sourceID); targetID != 0 {
		return targetID
	}
	return sourceID
}

// Has checks if a mapping exists for the given table and source ID
func (m *IDMapper) Has(table string, sourceID uint32) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.mappings[table] == nil {
		return false
	}
	_, exists := m.mappings[table][sourceID]
	return exists
}

// Clear removes all mappings for a specific table
func (m *IDMapper) Clear(table string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.mappings, table)
}

// ClearAll removes all mappings
func (m *IDMapper) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mappings = make(map[string]map[uint32]uint32)
}

// Count returns the number of mappings for a specific table
func (m *IDMapper) Count(table string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.mappings[table] == nil {
		return 0
	}
	return len(m.mappings[table])
}

// Tables returns the list of tables that have mappings
func (m *IDMapper) Tables() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tables := make([]string, 0, len(m.mappings))
	for table := range m.mappings {
		tables = append(tables, table)
	}
	return tables
}

// GetAllForTable returns all mappings for a specific table
func (m *IDMapper) GetAllForTable(table string) map[uint32]uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.mappings[table] == nil {
		return nil
	}

	// Return a copy to avoid concurrent modification
	result := make(map[uint32]uint32, len(m.mappings[table]))
	for k, v := range m.mappings[table] {
		result[k] = v
	}
	return result
}

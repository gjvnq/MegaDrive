package main

import "sync"

type map_uint64_string struct {
	raw  map[uint64]string
	lock *sync.RWMutex
}

func (m *map_uint64_string) Init() {
	m.raw = make(map[uint64]string)
	m.lock = &sync.RWMutex{}
}

func (m *map_uint64_string) Set(key uint64, val string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.raw[key] = val
}

func (m *map_uint64_string) Get(key uint64) string {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.raw[key]
}

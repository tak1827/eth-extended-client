package client

import (
	"sync"
)

type safeMap struct {
	sync.Mutex
	item map[string]struct{}
}

func (m *safeMap) add(hash string) {
	m.Lock()
	m.item[hash] = struct{}{}
	m.Unlock()
}

func (m *safeMap) delete(hash string) {
	m.Lock()
	delete(m.item, hash)
	m.Unlock()
}

func (m *safeMap) has(hash string) bool {
	m.Lock()
	_, exist := m.item[hash]
	m.Unlock()
	return exist
}

package main

import (
	"path/filepath"
	"sync"
)

func CGet2(key string) (interface{}, bool) {
	return MemCache.Get(key)
}

func CFound(keys ...string) bool {
	for _, key := range keys {
		if _, found := MemCache.Get(key); !found {
			Log.DebugNF(1, "Cache MISS for %s in %+v", key, keys)
			return false
		}
	}
	return true
}

func CFoundPrefix(prefix string, keys ...string) bool {
	for _, key := range keys {
		key = prefix + key
		if _, found := MemCache.Get(key); !found {
			Log.DebugNF(1, "Cache MISS for %s in %+v", key, keys)
			return false
		}
	}
	return true
}

func CGet(key string) interface{} {
	v, _ := MemCache.Get(key)
	return v
}

func CGetDef(key string, def interface{}) interface{} {
	v, f := MemCache.Get(key)
	if f {
		return v
	}
	return def
}

func CGetRWMutex(key string) *sync.RWMutex {
	v, f := MemCache.Get(key)
	if f {
		return v.(*sync.RWMutex)
	}
	// Create mutex
	mux := &sync.RWMutex{}
	MemCache.Set(key, mux, -1)
	return mux
}

func CUnlock(key string) {
	mux := CGetRWMutex(key)
	mux.Unlock()
}

func CRUnlock(key string) {
	mux := CGetRWMutex(key)
	mux.RUnlock()
}

func CRUnlockIf(key string, cond *bool) {
	if *cond {
		mux := CGetRWMutex(key)
		mux.RUnlock()
	}
}

func CLock(key string) {
	mux := CGetRWMutex(key)
	mux.Lock()
}

func CRLock(key string) {
	mux := CGetRWMutex(key)
	mux.RLock()
}

func CSet(key string, val interface{}) {
	MemCache.Set(key, val, 0)
}

func PathInCache(path ...string) string {
	tmp := make([]string, len(path)+1)
	tmp[0] = CacheDir
	for i, p := range path {
		tmp[i+1] = p
	}
	return filepath.Join(tmp...)
}

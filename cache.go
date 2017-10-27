package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
)

var StdBucket = []byte("generic")

func CFound(keys ...string) bool {
	tx, err := DB.Begin(false)
	if err != nil {
		Log.PanicF("Failed to create new transaction in database: %v", err)
	}
	defer tx.Rollback()

	b := tx.Bucket(StdBucket)
	for _, key := range keys {
		if v := b.Get([]byte(key)); v == nil {
			Log.DebugNF(1, "Cache MISS for %s in %+v", key, keys)
			return false
		}
	}
	return true
}

func CFoundPrefix(prefix string, keys ...string) bool {
	tx, err := DB.Begin(false)
	if err != nil {
		Log.PanicF("Failed to create new transaction in database: %v", err)
	}
	defer tx.Rollback()

	b := tx.Bucket(StdBucket)
	for _, key := range keys {
		key = prefix + key
		if v := b.Get([]byte(key)); v == nil {
			Log.DebugNF(1, "Cache MISS for %s in %+v", key, keys)
			return false
		}
	}
	return true
}

func CGet_bytes(key string) []byte {
	tx, err := DB.Begin(false)
	if err != nil {
		Log.PanicF("Failed to create new transaction in database: %v", err)
	}
	defer tx.Rollback()

	b := tx.Bucket(StdBucket)
	byt := b.Get([]byte(key))
	ret := make([]byte, len(byt))
	copy(ret, byt)
	return ret
}

func CGet(key string, dst interface{}) error {
	b := CGet_bytes(key)
	if b != nil {
		return json.Unmarshal(b, dst)
	}
	return nil
}

func CGet_str(key string) string {
	return string(CGet_bytes(key))
}

func CGet_int64(key string) int64 {
	v, err := strconv.ParseInt(CGet_str(key), 10, 64)
	if err != nil {
		Log.PanicF("Failed to parse %s as int (key %s): %v", v, key, err)
	}
	return v
}

func CGetDef_int64(key string, def int64) int64 {
	v, err := strconv.ParseInt(CGet_str(key), 10, 64)
	if err != nil {
		return def
	}
	return v
}

func CGetDef_uint64(key string, def uint64) uint64 {
	v, err := strconv.ParseUint(CGet_str(key), 10, 64)
	if err != nil {
		return def
	}
	return v
}

func CGetDef_uint32(key string, def uint32) uint32 {
	return uint32(CGetDef_uint64(key, uint64(def)))
}

func CGet_uint64(key string) uint64 {
	v, err := strconv.ParseUint(CGet_str(key), 10, 64)
	if err != nil {
		Log.PanicF("Failed to parse %s as uint (key %s): %v", v, key, err)
	}
	return v
}

func CGet_uint32(key string) uint32 {
	return uint32(CGet_uint64(key))
}

func CGet_bool(key string) bool {
	v, err := strconv.ParseBool(CGet_str(key))
	if err != nil {
		Log.PanicF("Failed to parse %s as bool (key %s): %v", v, key, err)
	}
	return v
}

func CGetDef_bool(key string, def bool) bool {
	v, err := strconv.ParseBool(CGet_str(key))
	if err != nil {
		return def
	}
	return v
}

func CGetDef(key string, dst interface{}, def interface{}) error {
	b := CGet_bytes(key)
	if b != nil {
		return json.Unmarshal(b, dst)
	}
	dst = def
	return nil
}

func CSet_bytes(key string, val []byte) {
	tx, err := DB.Begin(true)
	if err != nil {
		Log.PanicF("Failed to create new transaction in database: %v", err)
	}
	defer tx.Rollback()

	b := tx.Bucket(StdBucket)
	err = b.Put([]byte(key), val)
	if err != nil {
		Log.PanicF("Failed to save %s onto database: %v", key, err)
	}
	tx.Commit()
}

func CSet(key string, val interface{}) {
	byt, err := json.Marshal(val)
	if err != nil {
		Log.PanicF("Failed to encode json: %v", err)
	}
	CSet_bytes(key, byt)
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

func PathInCache(path ...string) string {
	tmp := make([]string, len(path)+1)
	tmp[0] = CacheDir
	for i, p := range path {
		tmp[i+1] = p
	}
	return filepath.Join(tmp...)
}

package main

import (
	"flag"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"sync"
	"time"

	"github.com/gjvnq/go-logger"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/patrickmn/go-cache"
	"github.com/syndtr/goleveldb/leveldb"
)

var RootNode = &MDNode{}
var DB *leveldb.DB
var FSConn *nodefs.FileSystemConnector
var FUSEServer *fuse.Server
var Inode2Id *map_uint64_string
var Unmounting bool
var CacheDir string
var MemCache *cache.Cache
var TheLogger *logger.Logger

func main() {
	main_fuse()
}

func CGet2(key string) (interface{}, bool) {
	return MemCache.Get(key)
}

func CFound(keys ...string) bool {
	for _, key := range keys {
		if _, found := MemCache.Get(key); !found {
			TheLogger.DebugNF(1, "Cache MISS for %s in %+v", key, keys)
			return false
		}
	}
	return true
}

func CFoundPrefix(prefix string, keys ...string) bool {
	for _, key := range keys {
		key = prefix + key
		if _, found := MemCache.Get(key); !found {
			TheLogger.DebugNF(1, "Cache MISS for %s in %+v", key, keys)
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

func PrintCallDuration(prefix string, start *time.Time) {
	elapsed := time.Since(*start)
	TheLogger.DebugNF(1, "%s: I took %s", prefix, elapsed)
}

func file_in_config(file string) (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	tokenCacheDir := filepath.Join(usr.HomeDir, ".config", "MegaDrive")
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir,
		url.QueryEscape(file)), err
}

func main_fuse() {
	var err error
	// Set a few variables
	RootNode.GoogleId = "root"

	// Set Logger
	TheLogger, err = logger.New("main", 1, os.Stdout)
	if err != nil {
		panic(err) // Check for error
	}

	// Get CLI options
	debug := flag.Bool("debug", false, "print debugging messages.")
	other := flag.Bool("allow-other", false, "mount with -o allowother.")
	flag.Parse()
	mount_point := flag.Arg(0)
	if len(flag.Args()) < 1 {
		TheLogger.FatalF("Usage:\n  MegaDrive MOUNTPOINT")
	}
	mount_point, _ = filepath.Abs(mount_point)
	mount_parent, _ := filepath.Abs(mount_point + "/..")

	// Get DB filepath
	db_file, err := file_in_config("level.db")
	if err != nil {
		TheLogger.FatalF("Failed to get database filepath: %v", err)
	}

	// Load DB
	DB, err = leveldb.OpenFile(db_file, nil)
	defer DB.Close()

	// Load Google Drive
	DriveClient = get_drive_client()

	// Prepare fs
	FSConn = nodefs.NewFileSystemConnector(RootNode, &nodefs.Options{})
	mOpts := &fuse.MountOptions{
		AllowOther: *other,
		Name:       "MegaDrive",
		FsName:     mount_point,
		Debug:      *debug,
	}
	os.Mkdir(mount_parent+"/.MegaDrive", 0755)
	CacheDir = mount_parent + "/.MegaDrive/"
	MemCache = cache.New(15*time.Minute, 30*time.Minute)

	// Mount fs
	FUSEServer, err = fuse.NewServer(FSConn.RawFS(), mount_point, mOpts)
	if err != nil {
		TheLogger.FatalF("Mount fail: %v", err)
	}

	// Prepare to deal with ctrl+c
	sig_chan := make(chan os.Signal, 20)
	signal.Notify(sig_chan, os.Interrupt)
	go func() {
		for _ = range sig_chan {
			Unmounting = true
			TheLogger.Notice("Unmounting...")
			FUSEServer.Unmount()
			FUSEServer.Unmount()
			FUSEServer.Unmount()
			TheLogger.Notice("Unmounted")
			os.Exit(0)
		}
	}()

	// Start consumers
	for i := 0; i < 3; i++ {
		go DriveGetBasicsConsumer()
		go DriveOpenDirConsumer()
	}
	// Pre Cache
	TheLogger.Notice("Pre-caching...")
	DriveGetBasics("root")
	// Start things
	TheLogger.Notice("Serving...")
	FUSEServer.Serve()
}

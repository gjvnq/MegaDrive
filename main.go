package main

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gjvnq/go-logger"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/patrickmn/go-cache"
)

var RootNode = &MDNode{}
var DB bolt.DB
var FSConn *nodefs.FileSystemConnector
var FUSEServer *fuse.Server
var Inode2Id *map_uint64_string
var Unmounting bool
var CacheDir string
var MemCache *cache.Cache
var Log *logger.Logger
var HackPoint *os.File

func PrintCallDuration(prefix string, start *time.Time) {
	elapsed := time.Since(*start)
	Log.DebugNF(1, "%s: I took %s", prefix, elapsed)
}

func main() {
	var err error
	// Set a few variables
	RootNode.GoogleId = "root"

	// Set Logger
	Log, err = logger.New("main", 1, os.Stdout)
	if err != nil {
		panic(err) // Check for error
	}

	// Get CLI options
	fuse_debug := flag.Bool("fuse-debug", false, "print debugging messages.")
	other := flag.Bool("allow-other", false, "mount with -o allowother.")
	flag.Parse()
	mount_point := flag.Arg(0)
	if len(flag.Args()) < 1 {
		Log.FatalF("Usage:\n  MegaDrive MOUNTPOINT")
	}
	mount_point, _ = filepath.Abs(mount_point)
	mount_base := filepath.Base(mount_point)
	mount_parent, _ := filepath.Abs(mount_point + "/..")

	// Prepare fs
	FSConn = nodefs.NewFileSystemConnector(RootNode, &nodefs.Options{})
	mOpts := &fuse.MountOptions{
		AllowOther: *other,
		Name:       "MegaDrive",
		FsName:     mount_point,
		Debug:      *fuse_debug,
	}
	CacheDir = mount_parent + "/.MegaDrive" + mount_base + "/"
	os.MkdirAll(CacheDir, 0755)
	os.MkdirAll(PathInCache("config"), 0755)
	os.MkdirAll(PathInCache("nodes"), 0755)
	MemCache = cache.New(15*time.Minute, 30*time.Minute)

	// Mount fs
	FUSEServer, err = fuse.NewServer(FSConn.RawFS(), mount_point, mOpts)
	if err != nil {
		Log.FatalF("Mount fail: %v", err)
	}

	// Load bolt
	DB, err := bolt.Open(CacheDir+"bolt.db", 0600, nil)
	if err != nil {
		Log.Fatal(err.Error())
	}
	defer DB.Close()

	// Load Google Drive
	DriveClient = GetDriveClient()

	// Prepare to deal with ctrl+c
	sig_chan := make(chan os.Signal, 20)
	signal.Notify(sig_chan, os.Interrupt)
	go func() {
		for _ = range sig_chan {
			Unmounting = true
			Log.Notice("Unmounting...")
			FUSEServer.Unmount()
			FUSEServer.Unmount()
			FUSEServer.Unmount()
			Log.Notice("Unmounted")
			os.Exit(0)
		}
	}()

	// Start consumers
	for i := 0; i < 3; i++ {
		go DriveGetBasicsConsumer()
		go DriveOpenDirConsumer()
		go DriveReadConsumer()
	}
	// Pre Cache
	Log.Notice("Pre-caching...")
	DriveGetBasics("root")
	// Start things
	Log.Notice("Serving...")
	FUSEServer.Serve()
}

package main

import (
	"flag"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/syndtr/goleveldb/leveldb"
	"log"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
)

var RootNode = &MDNode{}
var DB *leveldb.DB
var FSConn *nodefs.FileSystemConnector
var FUSEServer *fuse.Server
var Inode2Id *map_uint64_string
var Unmounting bool
var CacheDir string

func main() {
	main_fuse()
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
	// Inode2Id = &map_uint64_string{}
	// Inode2Id.Init()
	// Inode2Id.Set(RootNode, "root")

	// Set Logger
	log.SetFlags(log.Lmicroseconds)

	// Get CLI options
	debug := flag.Bool("debug", false, "print debugging messages.")
	other := flag.Bool("allow-other", false, "mount with -o allowother.")
	flag.Parse()
	mount_point := flag.Arg(0)
	if len(flag.Args()) < 1 {
		log.Fatal("Usage:\n  MegaDrive MOUNTPOINT")
	}
	mount_point, _ = filepath.Abs(mount_point)
	mount_parent, _ := filepath.Abs(mount_point + "/..")

	// Get DB filepath
	db_file, err := file_in_config("level.db")
	if err != nil {
		log.Fatalf("Failed to get database filepath: %v\n", err)
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

	// Mount fs
	FUSEServer, err = fuse.NewServer(FSConn.RawFS(), mount_point, mOpts)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}

	// Prepare to deal with ctrl+c
	sig_chan := make(chan os.Signal, 20)
	signal.Notify(sig_chan, os.Interrupt)
	go func() {
		for _ = range sig_chan {
			Unmounting = true
			log.Println("Unmounting...")
			FUSEServer.Unmount()
			FUSEServer.Unmount()
			FUSEServer.Unmount()
			log.Println("Unmounted")
			// os.Exit(0)
		}
	}()

	// Start things
	FUSEServer.Serve()
}

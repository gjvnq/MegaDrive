package main

import (
	"flag"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/syndtr/goleveldb/leveldb"
	"log"
	"os"
	"os/signal"
)

var RootNode = &MDNode{}
var DB *leveldb.DB
var FSConn *nodefs.FileSystemConnector
var FUSEServer *fuse.Server

func main() {
	main_fuse()
}

func main_fuse() {
	var err error

	// Set Logger
	log.SetFlags(log.Lmicroseconds)

	// Get CLI options
	debug := flag.Bool("debug", false, "print debugging messages.")
	other := flag.Bool("allow-other", false, "mount with -o allowother.")
	enableLinks := flag.Bool("l", false, "Enable hard link support")
	flag.Parse()
	mount_point := flag.Arg(1)
	if len(flag.Args()) < 1 {
		log.Fatal("Usage:\n  MegaDrive MOUNTPOINT")
	}

	// Mount DB
	DB, err = leveldb.OpenFile(".mega_drive/leveldb", nil)
	defer DB.Close()

	// Look for root inode
	data, err := DB.Get([]byte("map:google_id:to:inode:root"), nil)
	if err == nil {
		err = DB.Put([]byte("map:google_id:to:inode:root"), RootNode.Inode(), nil)
		if err != nil {
			log.Fatalf("Failed to write root inode: %v\n", err)
		}
	}

	// Prepare fs
	FSConn = nodefs.NewFileSystemConnector(RootNode, fuse.NewMountOptions())
	mount_point_abs, _ := filepath.Abs(mount_point)
	mOpts := &fuse.MountOptions{
		AllowOther: *other,
		Name:       "MegaDrive",
		FsName:     mount_point_abs,
		Debug:      *debug,
	}

	// Mount fs
	FUSEServer, err = fuse.NewServer(FSConn.RawFS(), mount_point, mOpts)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}

	// Prepare to deal with ctrl+c
	sig_chan := make(chan os.Signal, 1)
	signal.Notify(sig_chan, os.Interrupt)
	go func() {
		for _ = range sig_chan {
			log.Println("Unmounting...")
			FUSEServer.Unmount()
			log.Println("Unmounted")
		}
	}()

	// Start things
	FUSEServer.Serve()
}

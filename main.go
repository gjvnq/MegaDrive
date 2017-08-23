package main

import (
	"flag"
	"fmt"
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

	// Set Logger
	log.SetFlags(log.Lmicroseconds)

	// Get CLI options
	debug := flag.Bool("debug", false, "print debugging messages.")
	other := flag.Bool("allow-other", false, "mount with -o allowother.")
	flag.Parse()
	mount_point := flag.Arg(1)
	if len(flag.Args()) < 1 {
		log.Fatal("Usage:\n  MegaDrive MOUNTPOINT")
	}

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

	// Look for root node info
	tmp, _ := get_node_info("root")
	fmt.Printf("%+v\n", tmp)
	tmp2a, tmp2b := get_nodes_ids_with_parent("root")
	fmt.Printf("%+v %+v\n", tmp2a, tmp2b)
	// data, err := DB.Get([]byte("map:google_id:to:metadata:root"), nil)
	// if err == nil {
	// 	err = DB.Put([]byte("map:google_id:to:inode:root"), RootNode.Inode(), nil)
	// 	if err != nil {
	// 		log.Fatalf("Failed to write root inode: %v\n", err)
	// 	}
	// }

	// Prepare fs
	FSConn = nodefs.NewFileSystemConnector(RootNode, &nodefs.Options{})
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

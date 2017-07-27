package main

import (
	"flag"
	"log"
	// "github.com/syndtr/goleveldb/leveldb"
	// "github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

var RootNode = &MDNode{}

func main() {
	drive_test();
}

func main_fuse() {
	flag.Parse()
	if len(flag.Args()) < 1 {
		log.Fatal("Usage:\n  hello MOUNTPOINT")
	}
	server, _, err := nodefs.MountRoot(flag.Arg(0), RootNode, nil)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	server.Serve()
}
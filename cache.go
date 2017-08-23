package main

import (
	"log"
	"github.com/gogo/protobuf"
)

func save_to_db(key string, value interface{}) {
	// Encode in gob

	err = DB.Put([]byte(key), RootNode.Inode(), nil)
	if err != nil {
		log.Printf("Failed to write root inode: %v\n", err)
	}
}
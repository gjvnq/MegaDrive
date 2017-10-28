package main

import (
	"bufio"
	"os"
	"sync"
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

const READ_REFRESH_DELTA = 3 * time.Minute
const READ_CACHE_ENABLE = true
const READ_PRELOAD_ENABLE = false

var ChReadReq = make(chan string, 64)
var ChReadReqLP = make(chan string, 64)
var MapReadAns = make(map[string][]*sync.Mutex)
var MapReadAnsMux = new(sync.RWMutex)

// Adds the desired file id to ChReadReqLP if it is not full. Otherwise, nothing happens.
func DriveReadPreload(google_id string) {
	if READ_PRELOAD_ENABLE {
		select {
		case ChReadReqLP <- google_id:
		default:
		}
	}
}

// Adds the desired file id to ChReadReq and waits for the answer
func DriveRead(google_id string) fuse.Status {
	// Lock the MapReadAns for editing and add our own answer mutex
	MapReadAnsMux.Lock()
	mux := &sync.Mutex{}
	if _, b := MapReadAns[google_id]; !b {
		MapReadAns[google_id] = make([]*sync.Mutex, 0)
	}
	MapReadAns[google_id] = append(MapReadAns[google_id], mux)
	MapReadAnsMux.Unlock()
	// Tell the DriveReadConsumer to load this file's info
	ChReadReq <- google_id
	// Wait for it to finish (yes, it is a hack/gambiarra)
	mux.Lock()
	mux.Lock()
	Log.Notice(google_id)
	var status fuse.Status
	CGet("Read:"+google_id+":!ret", &status)
	return status
}

func file_mtime(path string) (mtime int64) {
	fi, err := os.Stat(path)
	if err != nil {
		return
	}
	mtime = fi.ModTime().Unix()
	return
}

func DriveReadConsumer() {
	Log.Notice("DriveReadConsumer: Started")
	for {
		google_id := ""
		select {
		case google_id = <-ChReadReq:
			Log.DebugF("DriveReadConsumer: Loaded %s from ChReadReq", google_id)
		case google_id = <-ChReadReqLP:
			Log.DebugF("DriveReadConsumer: Loaded %s from ChReadReqLP", google_id)
		}
		_start := time.Now()
		DriveGetBasics(google_id)
		// Check for some other goroutine also working on this
		flag_working := CGetDef_bool("Read:"+google_id+":!working", false)
		if flag_working == true {
			Log.DebugF("DriveReadConsumer: Skipping %s", google_id)
			continue
		}
		// Refresh file if the server version is newer
		cloud_mtime := CGetDef_int64("BasicAttr:"+google_id+":Mtime", 0)
		local_mtime := file_mtime(CacheDir + google_id)
		flag_refresh := cloud_mtime > local_mtime || cloud_mtime == 0 || local_mtime == 0
		if flag_refresh || !CFound("Read:"+google_id+":!ret") || READ_CACHE_ENABLE == false {
			DriveReadConsumerCore(google_id)
		}
		// Unlock answer mutexes
		MapReadAnsMux.Lock()
		for _, mux := range MapReadAns[google_id] {
			mux.Unlock()
		}
		MapReadAns[google_id] = make([]*sync.Mutex, 0)
		MapReadAnsMux.Unlock()
		Log.DebugF("DriveReadConsumer: unlocked mutexes for %s", google_id)
		PrintCallDuration("DriveReadConsumer", &_start)
	}
}

func DriveReadConsumerCore(google_id string) (ret_code fuse.Status) {
	// Save ourselves
	defer func() {
		if r := recover(); r != nil {
			Log.ErrorF("Recovered: %+v", r)
			ret_code = fuse.EIO
		}
	}()

	_start := time.Now()
	defer PrintCallDuration("DriveReadConsumerCore", &_start)
	Log.InfoF("DriveReadConsumerCore: Loading %s from the Internet", google_id)

	CSet("Read:"+google_id+":!ret", fuse.EIO)
	CSet("Read:"+google_id+":!working", true)
	defer CSet("Read:"+google_id+":!working", false)

	// Download file
	now := time.Now().Unix()
	CSet("Read:"+google_id+":!Mtime", now)
	r, err := DriveClient.Files.Get(google_id).Download()
	if err != nil {
		Log.ErrorF("Unable to Read %s: %v", google_id, err)
		return fuse.EIO
	}
	Log.InfoF("DriveReadConsumerCore: LOADED %s from the Internet", google_id)
	// Open file
	w, err := os.OpenFile(CacheDir+google_id, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		Log.ErrorF("Unable to Read %s: %v", google_id, err)
		return fuse.EIO
	}
	// Save file
	buf := bufio.NewReader(r.Body)
	_, err = buf.WriteTo(w)
	if err != nil {
		Log.ErrorF("Unable to Read %s: %v", google_id, err)
		return fuse.EIO
	}

	Log.InfoF("DriveReadConsumerCore: SAVED %s from the Internet on %s", google_id, CacheDir+google_id)
	CSet("Read:"+google_id+":!ret", fuse.OK)
	return fuse.OK
}

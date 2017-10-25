package main

import (
	"bufio"
	"os"
	"sync"
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

const UPDATE_DELTA_READ = 3 * time.Minute

var ChReadReq = make(chan string, 64)
var ChReadReqLP = make(chan string, 64)
var MapReadAns = make(map[string][]*sync.Mutex)
var MapReadAnsMux = new(sync.RWMutex)

// Adds the desired file id to ChReadReqLP if it is not full. Otherwise, nothing happens.
func DriveReadPreload(google_id string) {
	select {
	case ChReadReqLP <- google_id:
	default:
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
	TheLogger.Notice(google_id)
	return CGet("Read:" + google_id + ":!ret").(fuse.Status)
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
	TheLogger.Notice("DriveReadConsumer: Started")
	for {
		google_id := ""
		select {
		case google_id = <-ChReadReq:
			TheLogger.DebugF("DriveReadConsumer: Loaded %s from ChReadReq", google_id)
		case google_id = <-ChReadReqLP:
			TheLogger.DebugF("DriveReadConsumer: Loaded %s from ChReadReqLP", google_id)
		}
		_start := time.Now()
		DriveGetBasics(google_id)
		// Check for some other goroutine also working on this
		flag_working := CGetDef("Read:"+google_id+":!working", false).(bool) == true
		if flag_working {
			TheLogger.DebugF("DriveReadConsumer: Skipping %s", google_id)
			continue
		}
		// Refresh file if the server version is newer
		cloud_mtime := CGetDef("BasicAttr:"+google_id+":Mtime", uint64(0)).(uint64)
		local_mtime := file_mtime(CacheDir + google_id)
		flag_refresh := int64(cloud_mtime) > local_mtime || cloud_mtime == 0 || local_mtime == 0
		if flag_refresh || !CFound("Read:"+google_id+":!ret") {
			DriveReadConsumerCore(google_id)
		}
		// Unlock answer mutexes
		MapReadAnsMux.Lock()
		for _, mux := range MapReadAns[google_id] {
			mux.Unlock()
		}
		MapReadAns[google_id] = make([]*sync.Mutex, 0)
		MapReadAnsMux.Unlock()
		TheLogger.DebugF("DriveReadConsumer: unlocked mutexes for %s", google_id)
		PrintCallDuration("DriveReadConsumer", &_start)
	}
}

func DriveReadConsumerCore(google_id string) (ret_code fuse.Status) {
	// Save ourselves
	defer func() {
		if r := recover(); r != nil {
			TheLogger.ErrorF("Recovered: %+v", r)
			ret_code = fuse.EIO
		}
	}()

	_start := time.Now()
	defer PrintCallDuration("DriveReadConsumerCore", &_start)
	TheLogger.InfoF("DriveReadConsumerCore: Loading %s from the Internet", google_id)

	CSet("Read:"+google_id+":!ret", fuse.EIO)
	CSet("Read:"+google_id+":!working", true)
	defer CSet("Read:"+google_id+":!working", false)

	// Download file
	now := time.Now().Unix()
	CSet("Read:"+google_id+":!Mtime", now)
	r, err := DriveClient.Files.Get(google_id).Download()
	if err != nil {
		TheLogger.ErrorF("Unable to Read %s: %v", google_id, err)
		return fuse.EIO
	}
	TheLogger.InfoF("DriveReadConsumerCore: LOADED %s from the Internet", google_id)
	// Open file
	w, err := os.OpenFile(CacheDir+google_id, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		TheLogger.ErrorF("Unable to Read %s: %v", google_id, err)
		return fuse.EIO
	}
	// Save file
	buf := bufio.NewReader(r.Body)
	_, err = buf.WriteTo(w)
	if err != nil {
		TheLogger.ErrorF("Unable to Read %s: %v", google_id, err)
		return fuse.EIO
	}

	TheLogger.InfoF("DriveReadConsumerCore: SAVED %s from the Internet on %s", google_id, CacheDir+google_id)
	CSet("Read:"+google_id+":!ret", fuse.OK)
	return fuse.OK
}

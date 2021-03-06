package main

import (
	"sync"
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

const GETBASICS_REFRESH_DELTA = 3 * time.Minute
const GETBASICS_CACHE_ENABLE = true
const GETBASICS_PRELOAD_ENABLE = true

// When we need a new file's info, we add its id to ChBasicInfoReq which consumed ONLY by DriveGetBasicsConsumer. We also add our own (locked) mutex to MapBasicInfoAns. This way, whenever some function loads/reloads the piece of information we need, all functions waiting for it will have theirs mutexes unlocked, telling them that the information they need is now on the cache. DriveGetBasicsConsumer is smart enough to efficiently handle the same file id being multiple times on ChBasicInfoReq. LP means low priority and is used for preloading.
var ChBasicInfoReq = make(chan string, 64)
var ChBasicInfoReqLP = make(chan string, 64)
var MapBasicInfoAns = make(map[string][]*sync.Mutex)
var MapBasicInfoAnsMux = new(sync.RWMutex)

// Adds the desired file id to ChBasicInfoReqLP if it is not full. Otherwise, nothing happens.
func DriveGetBasicsPreload(google_id string) {
	if GETBASICS_PRELOAD_ENABLE {
		select {
		case ChBasicInfoReqLP <- google_id:
		default:
		}
	}
}

// Adds the desired file id to ChBasicInfoReq and waits for the answer
func DriveGetBasics(google_id string) fuse.Status {
	// Check for cached copy
	refresh_time := CGetDef_int64("BasicAttr:"+google_id+":!RefrehTime", 0)
	flag_ask_refresh := refresh_time < time.Now().Unix()
	flag_must_wait := refresh_time == 0 // Only wait for the answer when absolutely necessary

	// Ensure we try again in case of error
	ret := fuse.EIO
	CGet("BasicAttr:"+google_id+":!ret", &ret)
	if ret != fuse.OK {
		flag_ask_refresh = true
		flag_must_wait = true
	}

	if flag_ask_refresh || GETBASICS_CACHE_ENABLE == false {
		if flag_must_wait || GETBASICS_CACHE_ENABLE == false {
			// Lock the MapBasicInfoAns for editing and add our own answer mutex
			MapBasicInfoAnsMux.Lock()
			mux := &sync.Mutex{}
			if _, b := MapBasicInfoAns[google_id]; !b {
				MapBasicInfoAns[google_id] = make([]*sync.Mutex, 0)
			}
			MapBasicInfoAns[google_id] = append(MapBasicInfoAns[google_id], mux)
			MapBasicInfoAnsMux.Unlock()
			// Tell the DriveGetBasicsConsumer to load this file's info
			ChBasicInfoReq <- google_id
			// Wait for it to finish (yes, it is a hack/gambiarra)
			mux.Lock()
			mux.Lock()
		} else {
			// Tell the DriveGetBasicsConsumer to load this file's info
			// But do not wait for it
			ChOpenDirReq <- google_id
			Log.InfoF("%s (%s) will be refreshed later (async)", google_id, CGet_str("BasicAttr:"+google_id+":Name"))
		}
	}
	// Get return value
	ret = fuse.EIO
	CGet("BasicAttr:"+google_id+":!ret", &ret)
	if ret != fuse.OK {
		Log.WarningF("Failed to get basics for %s: %v", google_id, ret)
	}
	return ret
}

func DriveGetBasicsConsumer() {
	Log.Notice("DriveGetBasicsConsumer: Started")
	for {
		google_id := ""
		select {
		case google_id = <-ChBasicInfoReq:
			Log.DebugF("DriveGetBasicsConsumer: Loaded %s from ChBasicInfoReq", google_id)
		case google_id = <-ChBasicInfoReqLP:
			Log.DebugF("DriveGetBasicsConsumer: Loaded %s from ChBasicInfoReqLP", google_id)
		}
		_start := time.Now()
		// Do not work twice TODO: use timeout
		flag_working := CGetDef_bool("BasicAttr:"+google_id+":!working", false)
		if flag_working == true {
			Log.DebugF("DriveGetBasicsConsumer: Skipping %s", google_id)
			continue
		}
		// Actually work
		DriveGetBasicsConsumerCore(google_id)
		// Unlock answer mutexes
		MapBasicInfoAnsMux.Lock()
		for _, mux := range MapBasicInfoAns[google_id] {
			mux.Unlock()
		}
		MapBasicInfoAns[google_id] = make([]*sync.Mutex, 0)
		MapBasicInfoAnsMux.Unlock()
		Log.DebugF("DriveGetBasicsConsumer: unlocked mutexes for %s", google_id)
		PrintCallDuration("DriveGetBasicsConsumer", &_start)
	}
}

func DriveGetBasicsConsumerCore(google_id string) (ret_code fuse.Status) {
	// Save ourselves
	defer func() {
		if r := recover(); r != nil {
			Log.ErrorF("Recovered: %+v", r)
			ret_code = fuse.EIO
		}
	}()

	_start := time.Now()
	defer PrintCallDuration("DriveGetBasicsConsumerCore", &_start)
	Log.InfoF("DriveGetBasicsConsumerCore: Loading %s from the Internet", google_id)

	CSet("BasicAttr:"+google_id+":!working", true)
	defer CSet("BasicAttr:"+google_id+":!working", false)

	r, err := DriveClient.Files.Get(google_id).Fields("name, md5Checksum, modifiedTime, size, mimeType, createdTime").Do()
	if err != nil {
		Log.ErrorF("Unable to GetAttr %s: %v", google_id, err)
		CSet("BasicAttr:"+google_id+":!ret", fuse.EIO)
		return fuse.EIO
	}
	Log.InfoF("DriveGetBasicsConsumerCore: LOADED %s (%s) from the Internet", google_id, r.Name)

	// Set cache
	ret := DriveGetBasicsPut(google_id, r.Name, r.MimeType, r.Md5Checksum, r.Size, r.ModifiedTime, r.CreatedTime)
	return ret
}

func DriveGetBasicsPut(google_id string, name string, mimeType string, md5 string, size int64, modifiedTime string, createdTime string) fuse.Status {
	// Parse times
	mtime, err := time.Parse(time.RFC3339, modifiedTime)
	if err != nil {
		Log.ErrorF("Unable to GetAttr %s: %v", google_id, err)
		CSet("BasicAttr:"+google_id+":!ret", fuse.EIO)
		return fuse.EIO
	}
	ctime, err := time.Parse(time.RFC3339, createdTime)
	if err != nil {
		Log.ErrorF("Unable to GetAttr %s: %v", google_id, err)
		CSet("BasicAttr:"+google_id+":!ret", fuse.EIO)
		return fuse.EIO
	}
	// Lock file metadata
	CLock("BasicAttr:" + google_id + ":!mux")
	defer CUnlock("BasicAttr:" + google_id + ":!mux")

	// Save stuff
	CSet("BasicAttr:"+google_id+":!RefrehTime", time.Now().Add(GETBASICS_REFRESH_DELTA).Unix())
	CSet("BasicAttr:"+google_id+":Name", name)
	CSet("BasicAttr:"+google_id+":MimeType", mimeType)
	CSet("BasicAttr:"+google_id+":MD5", md5)
	CSet("BasicAttr:"+google_id+":IsDir", mimeType == "application/vnd.google-apps.folder")
	CSet("BasicAttr:"+google_id+":Size", uint64(size))
	CSet("BasicAttr:"+google_id+":Atime", uint64(mtime.Unix()))
	CSet("BasicAttr:"+google_id+":Ctime", uint64(ctime.Unix()))
	CSet("BasicAttr:"+google_id+":Mtime", uint64(mtime.Unix()))
	CSet("BasicAttr:"+google_id+":Atimensec", uint32(mtime.UnixNano()))
	CSet("BasicAttr:"+google_id+":Ctimensec", uint32(ctime.UnixNano()))
	CSet("BasicAttr:"+google_id+":Mtimensec", uint32(mtime.UnixNano()))
	CSet("BasicAttr:"+google_id+":!ret", fuse.OK)
	Log.InfoF("Updated BasicAttr for %s (%s)", google_id, name)
	return fuse.OK
}

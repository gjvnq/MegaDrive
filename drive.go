package main

import (
	"sync"
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

const GENERAL_UPDATE_DELTA = 15 * time.Minute

// When we need a new file's info, we add its id to ChBasicInfoReq which consumed ONLY by DriveGetBasicsConsumer. We also add our own (locked) mutex to MapBasicInfoAns. This way, whenever some function loads/reloads the piece of information we need, all functions waiting for it will have theirs mutexes unlocked, telling them that the information they need is now on the cache. DriveGetBasicsConsumer is smart enough to efficiently handle the same file id being multiple times on ChBasicInfoReq. LP means low priority and is used for preloading.
var ChBasicInfoReq = make(chan string, 64)
var ChBasicInfoReqLP = make(chan string, 64)
var MapBasicInfoAns = make(map[string][]*sync.Mutex)
var MapBasicInfoAnsMux = new(sync.RWMutex)

// When we need a new directories list, we add its id to ChOpenDirReq which consumed ONLY by DriveOpenDirConsumer. We also add our own (locked) mutex to MapOpenDirAns. This way, whenever some function loads/reloads the piece of information we need, all functions waiting for it will have theirs mutexes unlocked, telling them that the information they need is now on the cache. DriveOpenDirConsumer is smart enough to efficiently handle the same file id being multiple times on ChOpenDirReq. LP means low priority and is used for preloading.
var ChOpenDirReq = make(chan string, 64)
var ChOpenDirReqLP = make(chan string, 64)
var MapOpenDirAns = make(map[string][]*sync.Mutex)
var MapOpenDirAnsMux = new(sync.RWMutex)

// Adds the desired file id to ChBasicInfoReqLP if it is not full. Otherwise, nothing happens.
func DriveGetBasicsPreload(google_id string) {
	select {
	case ChBasicInfoReqLP <- google_id:
	default:
	}
}

// Adds the desired file id to ChBasicInfoReq and waits for the answer
func DriveGetBasics(google_id string) fuse.Status {
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
	return CGet("BasicAttr:" + google_id + ":!ret").(fuse.Status)
}

func DriveGetBasicsConsumer() {
	TheLogger.Notice("DriveGetBasicsConsumer: Started")
	for {
		google_id := ""
		select {
		case google_id = <-ChBasicInfoReq:
			TheLogger.DebugF("DriveGetBasicsConsumer: Loaded %s from ChBasicInfoReq", google_id)
		case google_id = <-ChBasicInfoReqLP:
			TheLogger.DebugF("DriveGetBasicsConsumer: Loaded %s from ChBasicInfoReqLP", google_id)
		}
		_start := time.Now()
		// Check for cached copy
		flag_working := CGetDef("BasicAttr:"+google_id+":!working", false).(bool) == true
		if flag_working {
			TheLogger.DebugF("DriveGetBasicsConsumer: Skipping %s", google_id)
			continue
		}
		flag_refresh := CGetDef("BasicAttr:"+google_id+":!RefrehTime", int64(0)).(int64) < time.Now().Unix()
		if flag_refresh {
			DriveGetBasicsConsumerCore(google_id)
		}
		// Unlock answer mutexes
		MapBasicInfoAnsMux.Lock()
		for _, mux := range MapBasicInfoAns[google_id] {
			mux.Unlock()
		}
		MapBasicInfoAns[google_id] = make([]*sync.Mutex, 0)
		MapBasicInfoAnsMux.Unlock()
		TheLogger.DebugF("DriveGetBasicsConsumer: unlocked mutexes for %s", google_id)
		PrintCallDuration("DriveGetBasicsConsumer", &_start)
	}
}

func DriveGetBasicsConsumerCore(google_id string) (ret_code fuse.Status) {
	// Save ourselves
	defer func() {
		if r := recover(); r != nil {
			TheLogger.ErrorF("Recovered: %+v", r)
			ret_code = fuse.EIO
		}
	}()

	_start := time.Now()
	defer PrintCallDuration("DriveGetBasicsConsumerCore", &_start)
	TheLogger.InfoF("DriveGetBasicsConsumerCore: Loading %s from the Internet", google_id)

	CSet("BasicAttr:"+google_id+":!working", true)
	defer CSet("BasicAttr:"+google_id+":!working", false)

	r, err := DriveClient.Files.Get(google_id).Fields("name, modifiedTime, size, mimeType, createdTime").Do()
	if err != nil {
		TheLogger.ErrorF("Unable to GetAttr %s: %v", google_id, err)
		CSet("BasicAttr:"+google_id+":!ret", fuse.EIO)
		return fuse.EIO
	}
	TheLogger.InfoF("DriveGetBasicsConsumerCore: LOADED %s (%s) from the Internet", google_id, r.Name)

	// Set cache
	ret := DriveGetBasicsPut(google_id, r.Name, r.MimeType, r.Size, r.ModifiedTime, r.CreatedTime)
	return ret
}

func DriveGetBasicsPut(google_id string, name string, mimeType string, size int64, modifiedTime string, createdTime string) fuse.Status {
	// Parse times
	mtime, err := time.Parse(time.RFC3339, modifiedTime)
	if err != nil {
		TheLogger.ErrorF("Unable to GetAttr %s: %v", google_id, err)
		CSet("BasicAttr:"+google_id+":!ret", fuse.EIO)
		return fuse.EIO
	}
	ctime, err := time.Parse(time.RFC3339, createdTime)
	if err != nil {
		TheLogger.ErrorF("Unable to GetAttr %s: %v", google_id, err)
		CSet("BasicAttr:"+google_id+":!ret", fuse.EIO)
		return fuse.EIO
	}
	// Lock file metadata
	CLock("BasicAttr:" + google_id + ":!mux")
	defer CUnlock("BasicAttr:" + google_id + ":!mux")

	// Save stuff
	CSet("BasicAttr:"+google_id+":!RefrehTime", time.Now().Add(GENERAL_UPDATE_DELTA).Unix())
	CSet("BasicAttr:"+google_id+":Name", name)
	CSet("BasicAttr:"+google_id+":MimeType", mimeType)
	CSet("BasicAttr:"+google_id+":IsDir", mimeType == "application/vnd.google-apps.folder")
	CSet("BasicAttr:"+google_id+":Size", uint64(size))
	CSet("BasicAttr:"+google_id+":Atime", uint64(mtime.Unix()))
	CSet("BasicAttr:"+google_id+":Ctime", uint64(ctime.Unix()))
	CSet("BasicAttr:"+google_id+":Mtime", uint64(mtime.Unix()))
	CSet("BasicAttr:"+google_id+":Atimensec", uint32(mtime.UnixNano()))
	CSet("BasicAttr:"+google_id+":Ctimensec", uint32(ctime.UnixNano()))
	CSet("BasicAttr:"+google_id+":Mtimensec", uint32(mtime.UnixNano()))
	CSet("BasicAttr:"+google_id+":!ret", fuse.OK)
	TheLogger.InfoF("Updated BasicAttr for %s (%s)", google_id, name)
	return fuse.OK
}

// Adds the desired file id to ChOpenDirReqLP if it is not full. Otherwise, nothing happens.
func DriveOpenDirPreload(google_id string) {
	select {
	case ChOpenDirReqLP <- google_id:
	default:
	}
}

// Adds the desired file id to ChOpenDirReq and waits for the answer
func DriveOpenDir(google_id string) ([]fuse.DirEntry, fuse.Status) {
	// Lock the MapBasicInfoAns for editing and add our own answer mutex
	MapOpenDirAnsMux.Lock()
	mux := &sync.Mutex{}
	if _, b := MapOpenDirAns[google_id]; !b {
		MapOpenDirAns[google_id] = make([]*sync.Mutex, 0)
	}
	MapOpenDirAns[google_id] = append(MapOpenDirAns[google_id], mux)
	MapOpenDirAnsMux.Unlock()
	// Tell the DriveGetBasicsConsumer to load this file's info
	ChOpenDirReq <- google_id
	// Wait for it to finish (yes, it is a hack/gambiarra)
	mux.Lock()
	mux.Lock()
	ans := CGet("OpenDir:" + google_id).([]fuse.DirEntry)
	status := CGet("OpenDir:" + google_id + ":!ret").(fuse.Status)
	return ans, status
}

func DriveOpenDirConsumer() {
	TheLogger.Notice("DriveOpenDirConsumer: Started")
	for {
		google_id := ""
		select {
		case google_id = <-ChOpenDirReq:
			TheLogger.DebugF("DriveOpenDirConsumer: Loaded %s from ChOpenDirReq", google_id)
		case google_id = <-ChOpenDirReqLP:
			TheLogger.DebugF("DriveOpenDirConsumer: Loaded %s from ChOpenDirReqLP", google_id)
		}
		_start := time.Now()
		// Check for cached copy
		flag_working := CGetDef("OpenDir:"+google_id+":!working", false).(bool) == true
		if flag_working {
			TheLogger.DebugF("DriveOpenDirConsumer: Skipping %s", google_id)
			continue
		}
		flag_refresh := CGetDef("OpenDir:"+google_id+":!RefrehTime", int64(0)).(int64) < time.Now().Unix()
		if flag_refresh {
			DriveOpenDirConsumerCore(google_id)
		}
		// Unlock answer mutexes
		MapOpenDirAnsMux.Lock()
		for _, mux := range MapOpenDirAns[google_id] {
			mux.Unlock()
		}
		MapOpenDirAns[google_id] = make([]*sync.Mutex, 0)
		MapOpenDirAnsMux.Unlock()
		TheLogger.DebugF("DriveOpenDirConsumer: unlocked mutexes for %s", google_id)
		PrintCallDuration("DriveOpenDirConsumer", &_start)
	}
}

func DriveOpenDirConsumerCore(google_id string) (ret_dirs []fuse.DirEntry, ret_code fuse.Status) {
	ret_dirs = make([]fuse.DirEntry, 0)
	ret_code = fuse.EIO

	// Save ourselves
	defer func() {
		if r := recover(); r != nil {
			TheLogger.ErrorF("Recovered: %+v", r)
			ret_code = fuse.EIO
		}
	}()

	_start := time.Now()
	defer PrintCallDuration("DriveOpenDirConsumerCore", &_start)
	TheLogger.InfoF("DriveOpenDirConsumerCore: Loading %s from the Internet", google_id)
	CSet("OpenDir:"+google_id+":!ret", fuse.EIO)

	CSet("OpenDir:"+google_id+":!working", true)
	defer CSet("OpenDir:"+google_id+":!working", false)

	// Call Google Drive
	r, err := DriveClient.Files.List().
		Fields("nextPageToken, files(id, name, modifiedTime, size, mimeType, createdTime)").
		Q(escape("'?' in parents and trashed = false", google_id)).
		Do()
	if err != nil {
		TheLogger.ErrorF("Unable to OpenDir %s: %v", google_id, err)
		return
	}
	name := CGetDef("BasicAttr:"+google_id+":Name", "?")
	TheLogger.InfoF("DriveOpenDirConsumerCore: LOADED %s (%s) from the Internet", google_id, name)

	ret_dirs = make([]fuse.DirEntry, 0)
	if len(r.Files) > 0 {
		// Return files found
		for _, file := range r.Files {
			val := fuse.DirEntry{}
			val.Name = file.Name
			isDir := file.MimeType == "application/vnd.google-apps.folder"
			if isDir {
				val.Mode = fuse.S_IFDIR
			}
			ret_dirs = append(ret_dirs, val)
			// Cache some stuff
			CSet("Lookup:"+file.Name+":in:"+google_id+":id", file.Id)
			CSet("Lookup:"+file.Name+":in:"+google_id+":isDir", isDir)
			// "Preload" some stuff to make things quicker
			go func() {
				found := CFoundPrefix("BasicAttr:"+google_id+":", "Name", "MimeType", "Size", "Atime", "Ctime", "Mtime", "Atimensec", "Ctimensec", "Mtimensec")
				if !found {
					DriveGetBasicsPut(file.Id, file.Name, file.MimeType, file.Size, file.ModifiedTime, file.CreatedTime)
					TheLogger.DebugF("Preloaded %s (%s)", file.Id, file.Name)
				}
			}()
		}
		// Save cache
		CSet("OpenDir:"+google_id, ret_dirs)
		CSet("OpenDir:"+google_id+":!ret", fuse.OK)
		CSet("OpenDir:"+google_id+":!RefrehTime", time.Now().Add(GENERAL_UPDATE_DELTA).Unix())
		ret_code = fuse.OK
		return
	} else {
		CSet("OpenDir:"+google_id+":!ret", fuse.ENODATA)
		ret_code = fuse.ENODATA
		return
	}
}

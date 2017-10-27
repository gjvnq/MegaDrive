package main

import (
	"sync"
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

const UPDATE_DELTA_OPENDIR = 3 * time.Minute

// When we need a new directories list, we add its id to ChOpenDirReq which consumed ONLY by DriveOpenDirConsumer. We also add our own (locked) mutex to MapOpenDirAns. This way, whenever some function loads/reloads the piece of information we need, all functions waiting for it will have theirs mutexes unlocked, telling them that the information they need is now on the cache. DriveOpenDirConsumer is smart enough to efficiently handle the same file id being multiple times on ChOpenDirReq. LP means low priority and is used for preloading.
var ChOpenDirReq = make(chan string, 64)
var ChOpenDirReqLP = make(chan string, 64)
var MapOpenDirAns = make(map[string][]*sync.Mutex)
var MapOpenDirAnsMux = new(sync.RWMutex)

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
	Log.Notice("DriveOpenDirConsumer: Started")
	for {
		google_id := ""
		select {
		case google_id = <-ChOpenDirReq:
			Log.DebugF("DriveOpenDirConsumer: Loaded %s from ChOpenDirReq", google_id)
		case google_id = <-ChOpenDirReqLP:
			Log.DebugF("DriveOpenDirConsumer: Loaded %s from ChOpenDirReqLP", google_id)
		}
		_start := time.Now()
		// Check for cached copy
		flag_working := CGetDef("OpenDir:"+google_id+":!working", false).(bool) == true
		if flag_working {
			Log.DebugF("DriveOpenDirConsumer: Skipping %s", google_id)
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
		Log.DebugF("DriveOpenDirConsumer: unlocked mutexes for %s", google_id)
		PrintCallDuration("DriveOpenDirConsumer", &_start)
	}
}

func DriveOpenDirConsumerCore(google_id string) (ret_dirs []fuse.DirEntry, ret_code fuse.Status) {
	ret_dirs = make([]fuse.DirEntry, 0)
	ret_code = fuse.EIO

	// Save ourselves
	defer func() {
		if r := recover(); r != nil {
			Log.ErrorF("Recovered: %+v", r)
			ret_code = fuse.EIO
		}
	}()

	_start := time.Now()
	defer PrintCallDuration("DriveOpenDirConsumerCore", &_start)
	Log.InfoF("DriveOpenDirConsumerCore: Loading %s from the Internet", google_id)
	CSet("OpenDir:"+google_id+":!ret", fuse.EIO)

	CSet("OpenDir:"+google_id+":!working", true)
	defer CSet("OpenDir:"+google_id+":!working", false)

	// Call Google Drive
	r, err := DriveClient.Files.List().
		Fields("nextPageToken, files(id, name, modifiedTime, size, md5Checksum, mimeType, createdTime)").
		Q(escape("'?' in parents and trashed = false", google_id)).
		Do()
	if err != nil {
		Log.ErrorF("Unable to OpenDir %s: %v", google_id, err)
		return
	}
	name := CGetDef("BasicAttr:"+google_id+":Name", "?")
	Log.InfoF("DriveOpenDirConsumerCore: LOADED %s (%s) from the Internet", google_id, name)

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
				found := CFoundPrefix("BasicAttr:"+google_id+":", "Name", "MimeType", "Size", "MD5", "Atime", "Ctime", "Mtime", "Atimensec", "Ctimensec", "Mtimensec")
				if !found {
					DriveGetBasicsPut(file.Id, file.Name, file.MimeType, file.Md5Checksum, file.Size, file.ModifiedTime, file.CreatedTime)
					Log.DebugF("Preloaded %s (%s)", file.Id, file.Name)
				}
			}()
		}
		// Save cache
		CSet("OpenDir:"+google_id, ret_dirs)
		CSet("OpenDir:"+google_id+":!ret", fuse.OK)
		CSet("OpenDir:"+google_id+":!RefrehTime", time.Now().Add(UPDATE_DELTA_OPENDIR).Unix())
		ret_code = fuse.OK
		return
	} else {
		CSet("OpenDir:"+google_id+":!ret", fuse.ENODATA)
		ret_code = fuse.ENODATA
		return
	}
}

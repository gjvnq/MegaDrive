package main

import (
	"sync"
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

const OPENDIR_REFRESH_DELTA = 3 * time.Minute
const OPENDIR_CACHE_ENABLE = true
const OPENDIR_PRELOAD_ENABLE = true
const OPENDIR_AUTO_CACHE_FOR_GETBASICS = true

// When we need a new directories list, we add its id to ChOpenDirReq which consumed ONLY by DriveOpenDirConsumer. We also add our own (locked) mutex to MapOpenDirAns. This way, whenever some function loads/reloads the piece of information we need, all functions waiting for it will have theirs mutexes unlocked, telling them that the information they need is now on the cache. DriveOpenDirConsumer is smart enough to efficiently handle the same file id being multiple times on ChOpenDirReq. LP means low priority and is used for preloading.
var ChOpenDirReq = make(chan string, 64)
var ChOpenDirReqLP = make(chan string, 64)
var MapOpenDirAns = make(map[string][]*sync.Mutex)
var MapOpenDirAnsMux = new(sync.RWMutex)

// Adds the desired file id to ChOpenDirReqLP if it is not full. Otherwise, nothing happens.
func DriveOpenDirPreload(google_id string) {
	if OPENDIR_PRELOAD_ENABLE {
		Log.DebugF("Preloading directory (new goroutine) %s", google_id)
		select {
		case ChOpenDirReqLP <- google_id:
		default:
		}
	}
}

// Adds the desired file id to ChOpenDirReq and waits for the answer
func DriveOpenDir(google_id string) ([]fuse.DirEntry, fuse.Status) {
	// Check for cached copy
	refresh_time := CGetDef_int64("OpenDir:"+google_id+":!RefrehTime", 0)
	flag_ask_refresh := refresh_time < time.Now().Unix()
	flag_must_wait := refresh_time == 0 // Only wait for the answer when absolutely necessary

	if flag_ask_refresh && OPENDIR_CACHE_ENABLE {
		if flag_must_wait || OPENDIR_CACHE_ENABLE == false {
			// Lock the MapBasicInfoAns for editing and add our own answer mutex
			MapOpenDirAnsMux.Lock()
			mux := &sync.Mutex{}
			if _, b := MapOpenDirAns[google_id]; !b {
				MapOpenDirAns[google_id] = make([]*sync.Mutex, 0)
			}
			MapOpenDirAns[google_id] = append(MapOpenDirAns[google_id], mux)
			mux.Lock()
			MapOpenDirAnsMux.Unlock()
			// Tell the DriveOpenDirConsumer to load this file's info
			ChOpenDirReq <- google_id
			// Wait for it to finish
			mux.Lock()
		} else {
			// Tell the DriveOpenDirConsumer to load this file's info
			// But do not wait for it
			ChOpenDirReq <- google_id
			Log.InfoF("%s will be refreshed later (async)", google_id)
		}
	}
	var ans []fuse.DirEntry
	var status fuse.Status
	CGet("OpenDir:"+google_id, &ans)
	CGet("OpenDir:"+google_id+":!ret", &status)
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
		// Avoid double work
		flag_working := CGetDef_bool("OpenDir:"+google_id+":!working", false)
		if flag_working == true {
			Log.DebugF("DriveOpenDirConsumer: Skipping %s", google_id)
			continue
		}
		// Actually work
		DriveOpenDirConsumerCore(google_id)
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
	name := CGet_str("BasicAttr:" + google_id + ":Name")
	Log.InfoF("DriveOpenDirConsumerCore: LOADED %s (%s) from the Internet", google_id, name)
	Log.InfoF("DriveOpenDirConsumerCore: %+v", r.Files)

	ret_dirs = make([]fuse.DirEntry, 0)
	if len(r.Files) > 0 {
		// Check for multiple files with the same name
		used_names := make(map[string]bool)
		doubled_names := make(map[string]bool)
		for _, file := range r.Files {
			n := MDNode{}
			n.GoogleId = file.Id
			n.Name = file.Name
			n.MimeType = file.MimeType

			name := n.SanitizedName()

			if used_names[name] == true {
				doubled_names[name] = true
			}
			used_names[name] = true
		}

		// Return files found
		for _, file := range r.Files {
			n := MDNode{}
			n.GoogleId = file.Id
			n.Name = file.Name
			n.MimeType = file.MimeType

			val := fuse.DirEntry{}
			val.Name = n.SanitizedName()
			// Be careful with files with equal names
			if doubled_names[val.Name] == true {
				val.Name = n.UnambiguousName()
			}
			if n.IsDir() {
				val.Mode = fuse.S_IFDIR
			}
			ret_dirs = append(ret_dirs, val)

			// Cache some stuff
			CSet("Lookup:"+val.Name+":in:"+google_id+":id", n.GoogleId)
			CSet("Lookup:"+val.Name+":in:"+google_id+":isDir", n.IsDir())
			// "Preload" some stuff to make things quicker
			if OPENDIR_AUTO_CACHE_FOR_GETBASICS {
				found := CFoundPrefix("BasicAttr:"+google_id+":", "Name", "MimeType", "Size", "MD5", "Atime", "Ctime", "Mtime", "Atimensec", "Ctimensec", "Mtimensec")
				if !found {
					DriveGetBasicsPut(file.Id, file.Name, file.MimeType, file.Md5Checksum, file.Size, file.ModifiedTime, file.CreatedTime)
					Log.DebugF("Preloaded %s (%s)", file.Id, file.Name)
				}
			}
		}
		// Save cache
		CSet("OpenDir:"+google_id, ret_dirs)
		CSet("OpenDir:"+google_id+":!ret", fuse.OK)
		CSet("OpenDir:"+google_id+":!RefrehTime", time.Now().Add(OPENDIR_REFRESH_DELTA).Unix())
		ret_code = fuse.OK
		return
	} else {
		CSet("OpenDir:"+google_id+":!ret", fuse.ENODATA)
		ret_code = fuse.ENODATA
		return
	}
}

func DriveOpenDirConsumerCoreBody() {
}

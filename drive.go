package main

import (
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

const GENERAL_UPDATE_DELTA = 15 * time.Minute

func DriveGetBasics(google_id string) fuse.Status {
	_start := time.Now()
	defer PrintCallDuration("DriveGetBasics", &_start)
	TheLogger.DebugF("DriveGetBasics %s", google_id)

	// Ensure we are the only ones analysing this file
	CLock("BasicAttr:" + google_id + ":!meta-mux")
	defer CUnlock("BasicAttr:" + google_id + ":!meta-mux")
	TheLogger.DebugF("DriveGetBasics: GOT lock %s", google_id)
	_start = time.Now()
	// Check for cache
	found := CFoundPrefix("BasicAttr:"+google_id+":", "Name", "MimeType", "Size", "Atime", "Ctime", "Mtime", "Atimensec", "Ctimensec", "Mtimensec")
	if found {
		// Ask for an update (async) if the time has come
		now := time.Now().Unix()
		if now >= CGetDef("BasicAttr:"+google_id+":!RefrehTime", 0).(int64) && CGet("BasicAttr:"+google_id+":!working").(bool) == false {
			TheLogger.InfoF("DriveGetBasics (refreshing) %s", google_id)
			go ActualDriveGetBasics(google_id)
		}
		return CGet("BasicAttr:" + google_id + ":!ret").(fuse.Status)
	}

	// Wait for it to finish
	return ActualDriveGetBasics(google_id)
}

func ActualDriveGetBasics(google_id string) fuse.Status {
	// Only one worker at a time
	if CGetDef("BasicAttr:"+google_id+":!working", false).(bool) == true {
		return CGet("BasicAttr:" + google_id + ":!ret").(fuse.Status)
	}

	// Tell other we are working
	CSet("BasicAttr:"+google_id+":!working", true)
	defer CSet("BasicAttr:"+google_id+":!working", false)
	CLock("BasicAttr:" + google_id + ":!w-mux")
	defer CUnlock("BasicAttr:" + google_id + ":!w-mux")

	_start := time.Now()
	defer PrintCallDuration("ActualDriveGetBasics", &_start)
	TheLogger.DebugF("ActualDriveGetBasics %s", google_id)

	r, err := DriveClient.Files.Get(google_id).Fields("name, modifiedTime, size, mimeType, createdTime").Do()
	if err != nil {
		TheLogger.ErrorF("Unable to GetAttr %s: %v", google_id, err)
		CSet("BasicAttr:"+google_id+":!ret", fuse.EIO)
		return fuse.EIO
	}

	// Set cache
	ActualDriveGetBasicsPut(google_id, r.Name, r.MimeType, r.Size, r.ModifiedTime, r.CreatedTime)
	return CGet("BasicAttr:" + google_id + ":!ret").(fuse.Status)
}

func ActualDriveGetBasicsPut(google_id string, name string, mimeType string, size int64, modifiedTime string, createdTime string) {
	_start := time.Now()
	defer PrintCallDuration("ActualDriveGetBasicsPut", &_start)

	// Parse times
	mtime, err := time.Parse(time.RFC3339, modifiedTime)
	if err != nil {
		TheLogger.ErrorF("Unable to GetAttr %s: %v", google_id, err)
		CSet("BasicAttr:"+google_id+":!ret", fuse.EIO)
		return
	}
	ctime, err := time.Parse(time.RFC3339, createdTime)
	if err != nil {
		TheLogger.ErrorF("Unable to GetAttr %s: %v", google_id, err)
		CSet("BasicAttr:"+google_id+":!ret", fuse.EIO)
		return
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
}

func DriveOpenDir(google_id string) (ret_dirs []fuse.DirEntry, ret_code fuse.Status) {
	_start := time.Now()
	defer PrintCallDuration("DriveOpenDir", &_start)

	ret_dirs = make([]fuse.DirEntry, 0)
	ret_code = fuse.EIO

	// Ensure we are the only ones analysing this file
	TheLogger.DebugF("DriveOpenDir %s", google_id)
	CLock("OpenDir:" + google_id + ":!meta-mux")
	defer CUnlock("OpenDir:" + google_id + ":!meta-mux")
	TheLogger.DebugF("DriveOpenDir: GOT lock %s", google_id)
	_start = time.Now()
	// Check for cache
	found := CFoundPrefix("OpenDir:"+google_id, "", ":!ret")
	if found {
		// Ask for an update (async) if the time has come
		now := time.Now().Unix()
		if now >= CGetDef("OpenDir:"+google_id+":!RefrehTime", 0).(int64) && CGetDef("OpenDir:"+google_id+":!working", false).(bool) == false {
			TheLogger.InfoF("DriveOpenDir (refreshing) %s", google_id)
			go ActualDriveOpenDir(google_id)
		}
		ret_dirs = CGet("OpenDir:" + google_id).([]fuse.DirEntry)
		ret_code = CGet("OpenDir:" + google_id + ":!ret").(fuse.Status)
		return
	}

	// Wait for it to finish
	return ActualDriveOpenDir(google_id)
}

func ActualDriveOpenDir(google_id string) (ret_dirs []fuse.DirEntry, ret_code fuse.Status) {
	ret_dirs = make([]fuse.DirEntry, 0)
	ret_code = fuse.EIO

	// Only one worker at a time
	if CGetDef("OpenDir:"+google_id+":!working", false).(bool) == true {
		ret_code = fuse.OK
		return
	}

	_start := time.Now()
	defer PrintCallDuration("ActualDriveOpenDir", &_start)

	// Tell other we are working
	CSet("OpenDir:"+google_id+":!working", true)
	defer CSet("OpenDir:"+google_id+":!working", false)
	CSet("OpenDir:"+google_id+":!ret", fuse.EIO)

	// Call Google Drive
	r, err := DriveClient.Files.List().
		Fields("nextPageToken, files(id, name, modifiedTime, size, mimeType, createdTime)").
		Q(escape("'?' in parents and trashed = false", google_id)).
		Do()
	if err != nil {
		TheLogger.ErrorF("Unable to OpenDir %s: %v", google_id, err)
		return
	}

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
			// Preload some stuff to make things quicker
			go func() {
				found := CFoundPrefix("BasicAttr:"+google_id+":", "Name", "MimeType", "Size", "Atime", "Ctime", "Mtime", "Atimensec", "Ctimensec", "Mtimensec")
				if !found {
					ActualDriveGetBasicsPut(file.Id, file.Name, file.MimeType, file.Size, file.ModifiedTime, file.CreatedTime)
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
		ret_code = fuse.ENODATA
		return
	}
}

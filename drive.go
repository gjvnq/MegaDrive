package main

import (
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

func DriveGetBasics(google_id string) fuse.Status {
	_start := time.Now()
	defer PrintCallDuration("DriveGetBasics", &_start)

	TheLogger.DebugF("DriveGetBasics %s", google_id)
	found := CFoundPrefix("basic-attr:"+google_id+":", "Name", "MimeType", "Size", "Atime", "Ctime", "Mtime", "Atimensec", "Ctimensec", "Mtimensec")
	if found {
		// TODO: Ask for an update (async)
		return fuse.OK
	}

	// Wait for it to finish
	return ActualDriveGetBasics(google_id)
}

func ActualDriveGetBasics(google_id string) fuse.Status {
	r, err := DriveClient.Files.Get(google_id).Fields("name, modifiedTime, size, mimeType, createdTime").Do()
	if err != nil {
		TheLogger.ErrorF("Unable to GetAttr %s: %v", google_id, err)
		CSet("basic-attr:"+google_id+"!ret", fuse.EIO)
		return fuse.EIO
	}
	mtime, err := time.Parse(time.RFC3339, r.ModifiedTime)
	if err != nil {
		TheLogger.ErrorF("Unable to GetAttr %s: %v", google_id, err)
		CSet("basic-attr:"+google_id+"!ret", fuse.EIO)
		return fuse.EIO
	}
	ctime, err := time.Parse(time.RFC3339, r.CreatedTime)
	if err != nil {
		TheLogger.ErrorF("Unable to GetAttr %s: %v", google_id, err)
		CSet("basic-attr:"+google_id+"!ret", fuse.EIO)
		return fuse.EIO
	}

	// Set cache
	CLock("basic-attr:" + google_id + "!mux")
	defer CUnlock("basic-attr:" + google_id + "!mux")
	CSet("basic-attr:"+google_id+":Name", r.Name)
	CSet("basic-attr:"+google_id+":MimeType", r.MimeType)
	CSet("basic-attr:"+google_id+":IsDir", r.MimeType == "application/vnd.google-apps.folder")
	CSet("basic-attr:"+google_id+":Size", uint64(r.Size))
	CSet("basic-attr:"+google_id+":Atime", uint64(mtime.Unix()))
	CSet("basic-attr:"+google_id+":Ctime", uint64(ctime.Unix()))
	CSet("basic-attr:"+google_id+":Mtime", uint64(mtime.Unix()))
	CSet("basic-attr:"+google_id+":Atimensec", uint32(mtime.UnixNano()))
	CSet("basic-attr:"+google_id+":Ctimensec", uint32(ctime.UnixNano()))
	CSet("basic-attr:"+google_id+":Mtimensec", uint32(mtime.UnixNano()))
	TheLogger.InfoF("Updated BasicAttr for %s (%s)", google_id, r.Name)

	CSet("basic-attr:"+google_id+"!ret", fuse.OK)
	return fuse.OK
}

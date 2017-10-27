package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

func escape(q string, args ...string) string {
	args2 := make([]interface{}, len(args))
	for i := range args {
		args2[i] = strings.Replace(args[i], "'", "\\'", -1)
	}
	q = strings.Replace(q, "'?'", "'%s'", -1)
	ret := fmt.Sprintf(q, args2...)
	Log.DebugF(ret)
	return ret
}

type MDNode struct {
	GoogleId   string
	inode      *nodefs.Inode
	Name       string
	MimeType   string
	MD5        string
	Size       uint64
	Atime      uint64
	Mtime      uint64
	Ctime      uint64
	Atimensec  uint32
	Mtimensec  uint32
	Ctimensec  uint32
	cache_file *os.File
}

func (n *MDNode) IsDir() bool {
	return n.MimeType == "application/vnd.google-apps.folder"
}

func (fs *MDNode) OnUnmount() {
	Log.DebugF("OnUnmount")
}

func (fs *MDNode) OnMount(conn *nodefs.FileSystemConnector) {
	Log.DebugF("OnMount")
}

func (n *MDNode) StatFs() *fuse.StatfsOut {
	Log.DebugF("StatFs")
	return nil
}

func (n *MDNode) SetInode(node *nodefs.Inode) {
	Log.DebugF("SetInode (%+v)", *node)
	n.inode = node
}

func (n *MDNode) Deletable() bool {
	Log.DebugF("Deletable")
	return true
}

func (n *MDNode) Inode() *nodefs.Inode {
	Log.DebugF("Inode (n=%v)", *n)
	return n.inode
}

func (n *MDNode) OnForget() {
	Log.DebugF("OnForget")
	if n.cache_file != nil {
		if err := n.cache_file.Close(); err != nil {
			Log.WarningF("Failed to close cache file for %s: %v", n.GoogleId, err)
		}
	}
}

func (n *MDNode) Lookup(out *fuse.Attr, name string, context *fuse.Context) (ret_node *nodefs.Inode, ret_code fuse.Status) {
	_start := time.Now()
	defer PrintCallDuration("Lookup", &_start)

	// Save ourselves
	defer func() {
		if r := recover(); r != nil {
			Log.ErrorF("Recovered: %+v", r)
			ret_node = &nodefs.Inode{}
			ret_code = fuse.EIO
		}
	}()

	Log.DebugF("Lookup (n=%v; out=%v; name=%v; context=%v)", *n, *out, name, *context)
	// Check for unmounting
	if Unmounting {
		Log.DebugF("Lookup ENODEV (Unmounting)")
		return nil, fuse.ENODEV
	}

	// Ensure data will be here
	go DriveOpenDir(n.GoogleId)

	// Check for cache
	if CFoundPrefix("Lookup:"+name+":in:"+n.GoogleId+":", "id", "isDir") {
		var isDir bool
		new_node := &MDNode{}
		new_node.GoogleId = CGet_str("Lookup:" + name + ":in:" + n.GoogleId + ":id")
		isDir = CGet_bool("Lookup:" + name + ":in:" + n.GoogleId + ":isDir")
		child := n.Inode().NewChild(name, isDir, new_node)
		child.Node().GetAttr(out, nil, context)
		Log.DebugF("%s -> fuse.OK", name)
		return child, fuse.OK
	}

	// Call Google Drive
	r, err := DriveClient.Files.List().
		Fields("files(id, mimeType)").
		Q(escape("'?' in parents and name = '?' and trashed = false", n.GoogleId, name)).
		Do()
	if err != nil {
		Log.ErrorF("Unable to LookUp %s in %s: %v", name, n.GoogleId, err)
		return nil, fuse.EIO
	}

	if len(r.Files) == 0 {
		Log.DebugF("%s -> fuse.ENOENT", name)
		return nil, fuse.ENOENT
	} else if len(r.Files) == 1 {
		new_node := &MDNode{}
		new_node.GoogleId = r.Files[0].Id
		isDir := (r.Files[0].MimeType == "application/vnd.google-apps.folder")
		Log.DebugF("%s -> fuse.OK", name)
		child := n.Inode().NewChild(name, isDir, new_node)
		child.Node().GetAttr(out, nil, context)

		// Save cache
		CSet("Lookup:"+name+":in:"+n.GoogleId+":id", new_node.GoogleId)
		CSet("Lookup:"+name+":in:"+n.GoogleId+":isDir", isDir)
		// Preload
		// Log.DebugF("Preloading (new goroutine) %s", new_node.GoogleId)
		// go n.GetBasics()
		// if isDir {
		// 	Log.DebugF("Preloading directory (new goroutine) %s", new_node.GoogleId)
		// 	go DriveOpenDir(new_node.GoogleId)
		// }

		return child, fuse.OK
	} else {
		Log.DebugF("%s -> fuse.EIO (%d) %+v", name, len(r.Files), r.Files)
		return nil, fuse.EIO
	}
}

func (n *MDNode) Access(mode uint32, context *fuse.Context) (code fuse.Status) {
	Log.DebugF("Access")
	return fuse.ENOSYS
}

func (n *MDNode) Readlink(c *fuse.Context) ([]byte, fuse.Status) {
	Log.DebugF("Readlink")
	return nil, fuse.ENOSYS
}

func (n *MDNode) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) (newNode *nodefs.Inode, code fuse.Status) {
	Log.DebugF("Mknod")
	return nil, fuse.ENOSYS
}
func (n *MDNode) Mkdir(name string, mode uint32, context *fuse.Context) (newNode *nodefs.Inode, code fuse.Status) {
	Log.DebugF("Mkdir")
	return nil, fuse.ENOSYS
}
func (n *MDNode) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	Log.DebugF("Unlink")
	return fuse.ENOSYS
}
func (n *MDNode) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	Log.DebugF("Rmdir")
	return fuse.ENOSYS
}
func (n *MDNode) Symlink(name string, content string, context *fuse.Context) (newNode *nodefs.Inode, code fuse.Status) {
	Log.DebugF("Symlink")
	return nil, fuse.ENOSYS
}

func (n *MDNode) Rename(oldName string, newParent nodefs.Node, newName string, context *fuse.Context) (code fuse.Status) {
	Log.DebugF("Rename")
	return fuse.ENOSYS
}

func (n *MDNode) Link(name string, existing nodefs.Node, context *fuse.Context) (newNode *nodefs.Inode, code fuse.Status) {
	Log.DebugF("Link")
	return nil, fuse.ENOSYS
}

func (n *MDNode) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, newNode *nodefs.Inode, code fuse.Status) {
	Log.DebugF("Create")
	return nil, nil, fuse.ENOSYS
}

func (n *MDNode) Open(flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	Log.DebugF("Open")
	return nil, fuse.OK
}

func (n *MDNode) Flush(file nodefs.File, openFlags uint32, context *fuse.Context) (code fuse.Status) {
	Log.DebugF("Flush")
	return fuse.ENOSYS
}

func (n *MDNode) OpenDir(context *fuse.Context) (ret_dirs []fuse.DirEntry, ret_code fuse.Status) {
	_start := time.Now()
	defer PrintCallDuration("OpenDir", &_start)

	// Save ourselves
	defer func() {
		if r := recover(); r != nil {
			Log.ErrorF("Recovered: %+v", r)
			ret_dirs = make([]fuse.DirEntry, 0)
			ret_code = fuse.EIO
		}
	}()

	Log.DebugF("OpenDir (n=%s, context=%v)", n.GoogleId, *context)
	// Check for unmounting
	if Unmounting {
		Log.DebugF("OpenDir ENODEV (Unmounting)")
		return nil, fuse.ENODEV
	}
	return DriveOpenDir(n.GoogleId)
}

func (n *MDNode) GetXAttr(attribute string, context *fuse.Context) (data []byte, code fuse.Status) {
	Log.DebugF("GetXAttr (attribute=%v)", attribute)
	if err := n.GetBasics(); err != fuse.OK {
		return nil, err
	}
	if attribute == "user.google-id" {
		return []byte(n.GoogleId), fuse.OK
	}
	return nil, fuse.ENOATTR
}

func (n *MDNode) RemoveXAttr(attr string, context *fuse.Context) fuse.Status {
	Log.DebugF("RemoveXAttr")
	return fuse.ENOSYS
}

func (n *MDNode) SetXAttr(attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	Log.DebugF("SetXAttr")
	return fuse.ENOSYS
}

func (n *MDNode) ListXAttr(context *fuse.Context) (attrs []string, code fuse.Status) {
	Log.DebugF("ListXAttr")
	return []string{"user.google-id"}, fuse.OK
}

func (n *MDNode) GetBasics() fuse.Status {
	_start := time.Now()
	defer PrintCallDuration("GetBasics", &_start)

	err := DriveGetBasics(n.GoogleId)
	if err != fuse.OK {
		return err
	}
	CRLock("BasicAttr:" + n.GoogleId + "!mux")
	defer CRUnlock("BasicAttr:" + n.GoogleId + "!mux")
	n.Name = CGet_str("BasicAttr:" + n.GoogleId + ":Name")
	n.MimeType = CGet_str("BasicAttr:" + n.GoogleId + ":MimeType")
	n.MD5 = CGet_str("BasicAttr:" + n.GoogleId + ":MD5")
	n.Size = CGetDef_uint64("BasicAttr:"+n.GoogleId+":Size", 0)
	n.Atime = CGetDef_uint64("BasicAttr:"+n.GoogleId+":Atime", 0)
	n.Ctime = CGetDef_uint64("BasicAttr:"+n.GoogleId+":Ctime", 0)
	n.Mtime = CGetDef_uint64("BasicAttr:"+n.GoogleId+":Mtime", 0)
	n.Atimensec = CGetDef_uint32("BasicAttr:"+n.GoogleId+":Atimensec", 0)
	n.Ctimensec = CGetDef_uint32("BasicAttr:"+n.GoogleId+":Ctimensec", 0)
	n.Mtimensec = CGetDef_uint32("BasicAttr:"+n.GoogleId+":Mtimensec", 0)
	return fuse.OK
}

func (n *MDNode) GetAttr(out *fuse.Attr, file nodefs.File, context *fuse.Context) (ret_code fuse.Status) {
	_start := time.Now()
	defer PrintCallDuration("GetAttr", &_start)

	// Save ourselves
	defer func() {
		if r := recover(); r != nil {
			Log.ErrorF("Recovered: %+v", r)
			ret_code = fuse.EIO
		}
	}()

	Log.DebugF("GetAttr (n=%v; out=%v; file=%v; context=%v)", *n, *out, file, *context)
	// Check for unmounting
	if Unmounting {
		Log.DebugF("Lookup GetAttr (Unmounting)")
		return fuse.ENODEV
	}
	if file != nil {
		return file.GetAttr(out)
	}
	// Basics first
	if n.Inode().IsDir() {
		out.Mode = fuse.S_IFDIR | 0755
	} else {
		out.Mode = fuse.S_IFREG | 0644
	}

	// Get size, dates, etc.
	if err := n.GetBasics(); err != fuse.OK {
		return err
	}

	// Preload
	if n.IsDir() {
		Log.DebugF("Preloading directory (new goroutine) %s", n.GoogleId)
		DriveOpenDirPreload(n.GoogleId)
	}

	out.Size = n.Size
	out.Atime = n.Atime
	out.Ctime = n.Ctime
	out.Mtime = n.Mtime
	out.Atimensec = n.Atimensec
	out.Ctimensec = n.Ctimensec
	out.Mtimensec = n.Mtimensec

	Log.DebugF("GetAttr %s -> ctime=%d mtime=%d mime=%s size=%d", n.GoogleId, out.Ctime, out.Mtime, n.MimeType, out.Size)
	return fuse.OK
}

func (n *MDNode) Chmod(file nodefs.File, perms uint32, context *fuse.Context) (code fuse.Status) {
	Log.DebugF("Chmod")
	return fuse.ENOSYS
}

func (n *MDNode) Chown(file nodefs.File, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	Log.DebugF("Chown")
	return fuse.ENOSYS
}

func (n *MDNode) Truncate(file nodefs.File, size uint64, context *fuse.Context) (code fuse.Status) {
	Log.DebugF("Truncate")
	return fuse.ENOSYS
}

func (n *MDNode) Utimens(file nodefs.File, atime *time.Time, mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	Log.DebugF("Utimens")
	return fuse.ENOSYS
}

func (n *MDNode) Fallocate(file nodefs.File, off uint64, size uint64, mode uint32, context *fuse.Context) (code fuse.Status) {
	Log.DebugF("Fallocate")
	return fuse.ENOSYS
}

func (n *MDNode) Read(file nodefs.File, dest []byte, off int64, context *fuse.Context) (ret_res fuse.ReadResult, ret_code fuse.Status) {
	_start := time.Now()
	defer PrintCallDuration("Read", &_start)
	var err error

	// Save ourselves
	defer func() {
		if r := recover(); r != nil {
			Log.ErrorF("Recovered: %+v", r)
			ret_res = nil
			ret_code = fuse.EIO
		}
	}()

	Log.DebugF("Read (len(dest)=%v off=%v context=%v", len(dest), off, context)
	sts := DriveRead(n.GoogleId)
	if sts != fuse.OK {
		Log.ErrorF("Unable to Read %s: %v", n.GoogleId, sts)
		return nil, sts
	}
	if n.cache_file != nil {
		if err := n.cache_file.Close(); err != nil {
			Log.WarningF("Unable to Read %s: failed to close previous file: %v", n.GoogleId, err)
		}
	}
	n.cache_file, err = os.Open(CacheDir + n.GoogleId)
	if err != nil {
		Log.ErrorF("Unable to Read %s: %v", n.GoogleId, err)
		return nil, fuse.EIO
	}
	return fuse.ReadResultFd(n.cache_file.Fd(), off, len(dest)), fuse.OK
}

func (n *MDNode) Write(file nodefs.File, data []byte, off int64, context *fuse.Context) (written uint32, code fuse.Status) {
	Log.DebugF("Write")
	// Check for unmounting
	if Unmounting {
		return 0, fuse.ENODEV
	}
	if file != nil {
		return file.Write(data, off)
	}
	return 0, fuse.ENOSYS
}

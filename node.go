package main

import (
	"fmt"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"log"
	"strings"
	"time"
)

func escape(q string, args ...string) string {
	args2 := make([]interface{}, len(args))
	for i := range args {
		args2[i] = strings.Replace(args[i], "'", "\\'", -1)
	}
	q = strings.Replace(q, "'?'", "'%s'", -1)
	ret := fmt.Sprintf(q, args2...)
	log.Println(ret)
	return ret
}

type MDNode struct {
	GoogleId string
	inode    *nodefs.Inode
}

func (fs *MDNode) OnUnmount() {
	fmt.Println("OnUnmount")
}

func (fs *MDNode) OnMount(conn *nodefs.FileSystemConnector) {
	fmt.Println("OnMount")
}

func (n *MDNode) StatFs() *fuse.StatfsOut {
	fmt.Println("StatFs")
	return nil
}

func (n *MDNode) SetInode(node *nodefs.Inode) {
	fmt.Println("SetInode")
	n.inode = node
}

func (n *MDNode) Deletable() bool {
	fmt.Println("Deletable")
	return true
}

func (n *MDNode) Inode() *nodefs.Inode {
	fmt.Printf("Inode (n=%v)\n", *n)
	return n.inode
}

func (n *MDNode) OnForget() {
	fmt.Println("OnForget")
}

func (n *MDNode) Lookup(out *fuse.Attr, name string, context *fuse.Context) (node *nodefs.Inode, code fuse.Status) {
	fmt.Printf("Lookup (n=%v; out=%v; name=%v; context=%v)\n", *n, *out, name, *context)
	// Check for unmounting
	if Unmounting {
		fmt.Printf("Lookup ENODEV (Unmounting)\n")
		return nil, fuse.ENODEV
	}

	// Call Google Drive
	r, err := DriveClient.Files.List().
		Fields("nextPageToken, files(id, name, mimeType)").
		Q(escape("'?' in parents and name = '?'", n.GoogleId, name)).
		Do()
	if err != nil {
		log.Printf("Unable to LookUp %s in %s: %v", name, n.GoogleId, err)
		return nil, fuse.EIO
	}

	if len(r.Files) == 0 {
		fmt.Printf("%s -> fuse.ENOENT", name)
		return nil, fuse.ENOENT
	} else if len(r.Files) == 1 {
		new_node := &MDNode{}
		new_node.GoogleId = r.Files[0].Id
		isDir := (r.Files[0].MimeType == "application/vnd.google-apps.folder")
		log.Printf("%s -> fuse.OK", name)
		return n.Inode().NewChild(name, isDir, new_node), fuse.OK
	} else {
		log.Printf("%s -> fuse.EIO (%d) %+v", name, len(r.Files), r.Files)
		return nil, fuse.EIO
	}
}

func (n *MDNode) Access(mode uint32, context *fuse.Context) (code fuse.Status) {
	fmt.Println("Access")
	return fuse.ENOSYS
}

func (n *MDNode) Readlink(c *fuse.Context) ([]byte, fuse.Status) {
	fmt.Println("Readlink")
	return nil, fuse.ENOSYS
}

func (n *MDNode) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) (newNode *nodefs.Inode, code fuse.Status) {
	fmt.Println("Mknod")
	return nil, fuse.ENOSYS
}
func (n *MDNode) Mkdir(name string, mode uint32, context *fuse.Context) (newNode *nodefs.Inode, code fuse.Status) {
	fmt.Println("Mkdir")
	return nil, fuse.ENOSYS
}
func (n *MDNode) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	fmt.Println("Unlink")
	return fuse.ENOSYS
}
func (n *MDNode) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	fmt.Println("Rmdir")
	return fuse.ENOSYS
}
func (n *MDNode) Symlink(name string, content string, context *fuse.Context) (newNode *nodefs.Inode, code fuse.Status) {
	fmt.Println("Symlink")
	return nil, fuse.ENOSYS
}

func (n *MDNode) Rename(oldName string, newParent nodefs.Node, newName string, context *fuse.Context) (code fuse.Status) {
	fmt.Println("Rename")
	return fuse.ENOSYS
}

func (n *MDNode) Link(name string, existing nodefs.Node, context *fuse.Context) (newNode *nodefs.Inode, code fuse.Status) {
	fmt.Println("Link")
	return nil, fuse.ENOSYS
}

func (n *MDNode) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, newNode *nodefs.Inode, code fuse.Status) {
	fmt.Println("Create")
	return nil, nil, fuse.ENOSYS
}

func (n *MDNode) Open(flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	fmt.Println("Open")
	return nil, fuse.ENOSYS
}

func (n *MDNode) Flush(file nodefs.File, openFlags uint32, context *fuse.Context) (code fuse.Status) {
	fmt.Println("Flush")
	return fuse.ENOSYS
}

func (n *MDNode) OpenDir(context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	fmt.Printf("OpenDir (n=%v, context=%v)\n", *n, *context)
	// Check for unmounting
	if Unmounting {
		fmt.Printf("OpenDir ENODEV (Unmounting)\n")
		return nil, fuse.ENODEV
	}
	// Call Google Drive
	r, err := DriveClient.Files.List().
		Fields("nextPageToken, files(id, name, mimeType)").
		Q(escape("'?' in parents", n.GoogleId)).
		Do()
	if err != nil {
		log.Printf("Unable to OpenDir %s: %v", n.GoogleId, err)
		return make([]fuse.DirEntry, 0), fuse.EIO
	}

	ret := make([]fuse.DirEntry, 0)
	if len(r.Files) > 0 {
		// Return files found
		for _, i := range r.Files {
			val := fuse.DirEntry{}
			val.Name = i.Name
			if i.MimeType == "application/vnd.google-apps.folder" {
				val.Mode = fuse.S_IFDIR
			}
			ret = append(ret, val)
		}
		return ret, fuse.OK
	} else {
		return make([]fuse.DirEntry, 0), fuse.ENODATA
	}
}

func (n *MDNode) GetXAttr(attribute string, context *fuse.Context) (data []byte, code fuse.Status) {
	fmt.Println("GetXAttr")
	return nil, fuse.ENOATTR
}

func (n *MDNode) RemoveXAttr(attr string, context *fuse.Context) fuse.Status {
	fmt.Println("RemoveXAttr")
	return fuse.ENOSYS
}

func (n *MDNode) SetXAttr(attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	fmt.Println("SetXAttr")
	return fuse.ENOSYS
}

func (n *MDNode) ListXAttr(context *fuse.Context) (attrs []string, code fuse.Status) {
	fmt.Println("ListXAttr")
	return nil, fuse.ENOSYS
}

func (n *MDNode) GetAttr(out *fuse.Attr, file nodefs.File, context *fuse.Context) (code fuse.Status) {
	fmt.Printf("GetAttr (n=%v; out=%v; file=%v; context=%v)\n", *n, *out, file, *context)
	// Check for unmounting
	if Unmounting {
		fmt.Printf("Lookup GetAttr (Unmounting)\n")
		return fuse.ENODEV
	}
	if file != nil {
		return file.GetAttr(out)
	}
	if n.Inode().IsDir() {
		out.Mode = fuse.S_IFDIR | 0755
	} else {
		out.Mode = fuse.S_IFREG | 0644
	}
	return fuse.OK
}

func (n *MDNode) Chmod(file nodefs.File, perms uint32, context *fuse.Context) (code fuse.Status) {
	fmt.Println("Chmod")
	return fuse.ENOSYS
}

func (n *MDNode) Chown(file nodefs.File, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	fmt.Println("Chown")
	return fuse.ENOSYS
}

func (n *MDNode) Truncate(file nodefs.File, size uint64, context *fuse.Context) (code fuse.Status) {
	fmt.Println("Truncate")
	return fuse.ENOSYS
}

func (n *MDNode) Utimens(file nodefs.File, atime *time.Time, mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	fmt.Println("Utimens")
	return fuse.ENOSYS
}

func (n *MDNode) Fallocate(file nodefs.File, off uint64, size uint64, mode uint32, context *fuse.Context) (code fuse.Status) {
	fmt.Println("Fallocate")
	return fuse.ENOSYS
}

func (n *MDNode) Read(file nodefs.File, dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
	fmt.Println("Read")
	if file != nil {
		return file.Read(dest, off)
	}
	return nil, fuse.ENOSYS
}

func (n *MDNode) Write(file nodefs.File, data []byte, off int64, context *fuse.Context) (written uint32, code fuse.Status) {
	fmt.Println("Write")
	// Check for unmounting
	if Unmounting {
		return 0, fuse.ENODEV
	}
	if file != nil {
		return file.Write(data, off)
	}
	return 0, fuse.ENOSYS
}

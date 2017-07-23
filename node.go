package main

import (
	"time"
	"fmt"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

type MDNode struct {
	inode *nodefs.Inode
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
	fmt.Println("Lookup")
	return nil, fuse.ENOENT
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
	fmt.Printf("OpenDir (context=%v)\n", *context)
	return make([]fuse.DirEntry, 0), fuse.ENOSYS
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

func (n *MDNode) GetAttr(out *fuse.Attr, file nodefs.File, context *fuse.Context) (code 	fuse.Status) {
	fmt.Printf("GetAttr (n=%v; out=%v; file=%v; context=%v)\n", *n, *out, file, *context)
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
	if file != nil {
		return file.Write(data, off)
	}
	return 0, fuse.ENOSYS
}
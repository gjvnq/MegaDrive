MNT=/var/tmp/mnt0

.PHONY: run all

all: MegaDrive

dev: MegaDrive
	-fusermount -u $(MNT)
	./MegaDrive $(MNT)

MegaDrive: *.go
	goimports -w *.go
	go fmt
	go build
package tree

import (
	"bufio"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// The snapshot format: a compact, compressed encoding of what a Scan saw
// (Disks, Folders, files), without the figures Finish computes, which
// Decode computes afresh. Version 1:
//
//	"HBT" 0x01, then deflate of:
//	  uvarint #disks, then each disk name
//	  the root folder, where a folder is:
//	    name, uvarint #files, each file, uvarint #folders, each folder
//	  and a file is:
//	    name, uvarint disk, uvarint dev, uvarint ino, uvarint links, varint bytes
//
// A string is a uvarint length and its bytes. To add a field, add a new
// version rather than change this one: a snapshot written by an older
// hoardboard must still load.
var magic = []byte{'H', 'B', 'T', 1}

const (
	maxName  = 4096 // longer than any name Linux allows (255 bytes)
	maxDepth = 4096
)

// Encode writes t in the snapshot format.
func Encode(w io.Writer, t *Tree) error {
	if _, err := w.Write(magic); err != nil {
		return err
	}
	zw, err := flate.NewWriter(w, flate.DefaultCompression)
	if err != nil {
		return err
	}
	e := encoder{w: bufio.NewWriterSize(zw, 64*1024)}
	e.uvarint(uint64(len(t.Disks)))
	for _, d := range t.Disks {
		e.string(d)
	}
	e.folder(t.Root)
	if e.err != nil {
		return e.err
	}
	if err := e.w.Flush(); err != nil {
		return err
	}
	return zw.Close()
}

type encoder struct {
	w   *bufio.Writer
	buf [binary.MaxVarintLen64]byte
	err error
}

func (e *encoder) uvarint(v uint64) {
	if e.err == nil {
		_, e.err = e.w.Write(binary.AppendUvarint(e.buf[:0], v))
	}
}

func (e *encoder) varint(v int64) {
	if e.err == nil {
		_, e.err = e.w.Write(binary.AppendVarint(e.buf[:0], v))
	}
}

func (e *encoder) string(s string) {
	e.uvarint(uint64(len(s)))
	if e.err == nil {
		_, e.err = e.w.WriteString(s)
	}
}

func (e *encoder) folder(f *Folder) {
	e.string(f.Name)
	e.uvarint(uint64(len(f.Files)))
	for _, file := range f.Files {
		e.string(file.Name)
		e.uvarint(uint64(file.Disk))
		e.uvarint(file.Dev)
		e.uvarint(file.Ino)
		e.uvarint(uint64(file.Links))
		e.varint(file.Bytes)
	}
	e.uvarint(uint64(len(f.Folders)))
	for _, c := range f.Folders {
		e.folder(c)
	}
}

// Decode reads a Tree written by Encode and finishes it.
func Decode(r io.Reader) (*Tree, error) {
	head := make([]byte, len(magic))
	if _, err := io.ReadFull(r, head); err != nil {
		return nil, fmt.Errorf("snapshot: %w", err)
	}
	if string(head[:3]) != string(magic[:3]) {
		return nil, errors.New("snapshot: not a tree snapshot")
	}
	if head[3] != magic[3] {
		return nil, fmt.Errorf("snapshot: version %d, this build reads %d", head[3], magic[3])
	}
	zr := flate.NewReader(r)
	defer zr.Close()
	d := decoder{r: bufio.NewReaderSize(zr, 64*1024), b: NewBuilder()}
	n := d.uvarint()
	if n > 1<<16 {
		return nil, errors.New("snapshot: too many disks")
	}
	for range n {
		d.b.Disk(d.string())
	}
	if name := d.string(); d.err == nil && name != "" {
		return nil, errors.New("snapshot: root folder has a name")
	}
	d.folder(d.b.Root(), 0)
	if d.err != nil {
		return nil, fmt.Errorf("snapshot: %w", d.err)
	}
	return d.b.Finish(), nil
}

type decoder struct {
	r   *bufio.Reader
	b   *Builder
	err error
}

func (d *decoder) uvarint() uint64 {
	if d.err != nil {
		return 0
	}
	v, err := binary.ReadUvarint(d.r)
	d.err = noEOF(err)
	return v
}

func (d *decoder) varint() int64 {
	if d.err != nil {
		return 0
	}
	v, err := binary.ReadVarint(d.r)
	d.err = noEOF(err)
	return v
}

func (d *decoder) string() string {
	n := d.uvarint()
	if d.err != nil {
		return ""
	}
	if n > maxName {
		d.err = errors.New("name too long")
		return ""
	}
	buf := make([]byte, n)
	_, err := io.ReadFull(d.r, buf)
	d.err = noEOF(err)
	return string(buf)
}

// folder reads into's files and subfolders (its name is already read).
func (d *decoder) folder(into *Folder, depth int) {
	if depth > maxDepth {
		d.err = errors.New("folders nested too deep")
		return
	}
	files := d.uvarint()
	for i := uint64(0); i < files && d.err == nil; i++ {
		f := File{Name: d.string()}
		disk := d.uvarint()
		f.Dev = d.uvarint()
		f.Ino = d.uvarint()
		links := d.uvarint()
		f.Bytes = d.varint()
		if d.err == nil && (disk >= uint64(len(d.b.t.Disks)) || links > 1<<32-1) {
			d.err = errors.New("bad file")
		}
		f.Disk, f.Links = DiskID(disk), uint32(links)
		if d.err == nil {
			d.b.File(into, f)
		}
	}
	folders := d.uvarint()
	for i := uint64(0); i < folders && d.err == nil; i++ {
		name := d.string()
		if d.err == nil {
			d.folder(d.b.Folder(into, name), depth+1)
		}
	}
}

func noEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

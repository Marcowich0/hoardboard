// Package inventory knows which Disks the array has and how full they are.
//
// It reads Unraid's status folder (disks.ini, written by emhttpd) to list the
// data disks, the cache pools and parity, and asks each Disk's filesystem for
// its live Used space and Free space. Nothing is cached: every call reads the
// files and the filesystems again, so a Disk Unraid newly lists shows up on
// the next call, with no restart.
package inventory

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Inventory reads the array's Disks. Its zero value is not usable: see New.
type Inventory struct {
	mountRoot string
	statusDir string
}

// New returns an Inventory that finds each Disk at <mountRoot>/<name> and
// reads Unraid's status files from statusDir.
func New(mountRoot, statusDir string) *Inventory {
	return &Inventory{mountRoot: mountRoot, statusDir: statusDir}
}

// Kind is what a Disk is to the array.
type Kind string

const (
	Data  Kind = "data"  // an array data disk (disk1, disk2, …)
	Cache Kind = "cache" // a pool, whatever its name
)

// Disk is one mounted data disk or pool: something with a filesystem of its
// own that holds Share folders.
type Disk struct {
	Name       string // "disk1", "cache"
	Kind       Kind
	Filesystem string // "xfs", "btrfs"
	// Path is the Disk's folder: <mount root>/<name>.
	Path string
}

// Space is what a filesystem reports, in bytes. Used+Free can be less than
// Size where the filesystem reserves blocks for root.
type Space struct {
	Size int64
	Used int64
	Free int64
}

// Array is what Unraid lists right now.
type Array struct {
	// Disks are the mounted data disks and pools, in disks.ini's order.
	// Empty slots, unmounted disks and the flash drive are left out.
	Disks []Disk
	// Parity are the assigned parity disks, for information only: they hold
	// no files and are never usable space.
	Parity []Parity
}

// Parity is one assigned parity disk.
type Parity struct {
	Name   string // "parity", "parity2"
	Size   int64  // bytes
	Status ParityStatus
	// LastCheck is when the last parity operation finished, zero if Unraid
	// doesn't say.
	LastCheck time.Time
}

// ParityStatus is how a parity disk stands, from disks.ini's status.
type ParityStatus string

const (
	ParityValid   ParityStatus = "valid"   // DISK_OK
	ParityInvalid ParityStatus = "invalid" // disabled, invalid, wrong or new
	ParityMissing ParityStatus = "missing" // assigned but not present
)

// Read lists the array's Disks and parity from disks.ini, and when parity
// was last checked from var.ini.
func (inv *Inventory) Read() (Array, error) {
	secs, err := readINI(filepath.Join(inv.statusDir, "disks.ini"))
	if err != nil {
		return Array{}, fmt.Errorf("disk inventory: %w", err)
	}
	vars, err := inv.vars()
	if err != nil {
		return Array{}, err
	}
	var lastCheck time.Time
	if n := vars.int("sbSynced2"); n > 0 {
		lastCheck = time.Unix(n, 0)
	}

	var a Array
	seen := map[string]bool{}
	for _, s := range secs {
		name := s.vals["name"]
		if name == "" {
			name = s.name
		}
		var kind Kind
		switch s.vals["type"] {
		case "Data":
			kind = Data
		case "Cache":
			kind = Cache
		case "Parity":
			if p, ok := parity(name, s); ok {
				p.LastCheck = lastCheck
				a.Parity = append(a.Parity, p)
			}
			continue
		default:
			continue
		}
		// An empty slot has a mount point but is not mounted. A pool with
		// several devices may list each member; they share one mount point.
		mp := s.vals["fsMountpoint"]
		if s.vals["fsStatus"] != "Mounted" || mp == "" || seen[mp] || !validName(name) {
			continue
		}
		seen[mp] = true
		a.Disks = append(a.Disks, Disk{
			Name:       name,
			Kind:       kind,
			Filesystem: s.vals["fsType"],
			Path:       filepath.Join(inv.mountRoot, name),
		})
	}
	return a, nil
}

// parity reads a Parity section; ok is false for an empty slot.
func parity(name string, s section) (Parity, bool) {
	p := Parity{Name: name, Size: s.int("size") * 1024}
	switch status := s.vals["status"]; {
	case status == "DISK_OK":
		p.Status = ParityValid
	case status == "DISK_NP_MISSING":
		p.Status = ParityMissing
	case strings.HasPrefix(status, "DISK_NP"):
		return Parity{}, false // DISK_NP, DISK_NP_DSBL: nothing assigned
	default:
		p.Status = ParityInvalid
	}
	return p, true
}

// vars reads var.ini, the array's state. A missing file reads as empty.
func (inv *Inventory) vars() (section, error) {
	secs, err := readINI(filepath.Join(inv.statusDir, "var.ini"))
	if errors.Is(err, fs.ErrNotExist) {
		return section{vals: map[string]string{}}, nil
	}
	if err != nil {
		return section{}, fmt.Errorf("disk inventory: %w", err)
	}
	return secs[0], nil
}

// validName keeps a name from disks.ini from pointing outside the mount root.
func validName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\`)
}

// Space asks the Disk's filesystem how full it is, live. It reads no file.
func (d Disk) Space() (Space, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(d.Path, &st); err != nil {
		return Space{}, fmt.Errorf("disk %s: %w", d.Name, err)
	}
	bs := int64(st.Frsize)
	if bs == 0 {
		bs = int64(st.Bsize)
	}
	return Space{
		Size: int64(st.Blocks) * bs,
		Used: int64(st.Blocks-st.Bfree) * bs,
		Free: int64(st.Bavail) * bs,
	}, nil
}

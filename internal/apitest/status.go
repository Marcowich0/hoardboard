package apitest

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Status is a fake Unraid status folder (/var/local/emhttp) holding disks.ini
// and var.ini in emhttpd's format: `["section"]` headers and `key="value"`
// lines. NewStatus describes an idle, started array shaped like the owner's:
// parity, disk1, disk2, an empty disk3 slot, an empty parity2 slot, the cache
// pool and the flash drive.
//
// Like the real server, a Disk's fsMountpoint is /mnt/<name> whatever the
// mount root: the app must find a Disk's folder as <mount root>/<name>.
//
// Every change rewrites the file at once (a temp file renamed over it), as the
// app must read the files fresh and never see half of one.
type Status struct {
	Dir string

	t     testing.TB
	mu    sync.Mutex
	disks []section         // disks.ini, in file order
	vars  map[string]string // var.ini
	order []string          // var.ini key order
}

type section struct {
	name string
	keys []string
	vals map[string]string
}

// Sizes of the default Disks, in KiB as in disks.ini (Tower's real values).
const (
	parityKiB = 11718885324
	dataKiB   = 3907018532
	cacheKiB  = 2000397656
)

// NewStatus makes a status folder for an idle, started array.
func NewStatus(t testing.TB) *Status {
	t.Helper()
	s := &Status{Dir: filepath.Join(t.TempDir(), "emhttp"), t: t, vars: map[string]string{}}
	if err := os.Mkdir(s.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s.disks = []section{
		newSection("parity", "idx", "0", "device", "sda", "size", kib(parityKiB),
			"status", "DISK_OK", "type", "Parity", "color", "green-on", "rotational", "1"),
		dataDisk("disk1", 1, "sdb"),
		dataDisk("disk2", 2, "sdc"),
		newSection("disk3", "idx", "3", "device", "", "size", "0", "status", "DISK_NP",
			"type", "Data", "color", "grey-off", "fsType", "auto", "fsStatus", "-",
			"fsMountpoint", "/mnt/disk3"),
		newSection("parity2", "idx", "29", "device", "", "size", "0",
			"status", "DISK_NP_DSBL", "type", "Parity", "color", "grey-off"),
		newSection("cache", "idx", "30", "device", "nvme0n1", "size", kib(cacheKiB),
			"status", "DISK_OK", "type", "Cache", "color", "green-on", "rotational", "0",
			"transport", "nvme", "fsType", "btrfs", "fsStatus", "Mounted",
			"fsMountpoint", "/mnt/cache", "fsSize", kib(cacheKiB), "fsUsed", "0",
			"fsFree", kib(cacheKiB), "state", "STARTED"),
		newSection("flash", "idx", "31", "device", "sdh", "size", "30718976",
			"status", "DISK_OK", "type", "Flash", "color", "green-on", "fsType", "vfat",
			"fsStatus", "Mounted", "fsMountpoint", "/boot"),
	}
	for _, kv := range [][2]string{
		{"version", "7.3.4"},
		{"mdState", "STARTED"},
		{"fsState", "Started"},
		{"mdResync", "0"},
		{"mdResyncPos", "0"},
		{"mdResyncAction", "check P"},
		{"mdResyncSize", kib(parityKiB)},
		{"mdResyncDt", "0"},
		{"mdResyncDb", "0"},
		{"shareMoverActive", "no"},
		{"shareMoverSchedule", "0 0 * * *"},
		{"spindownDelay", "0"},
	} {
		s.order = append(s.order, kv[0])
		s.vars[kv[0]] = kv[1]
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeDisks()
	s.writeVars()
	return s
}

// AddDataDisk lists a new, mounted data Disk, as when the owner adds one to
// the array. Pair it with Mount.AddDisk. An empty slot of the same name (such
// as disk3) is filled in.
func (s *Status) AddDataDisk(name string) {
	s.t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := len(s.disks)
	if n, err := strconv.Atoi(strings.TrimPrefix(name, "disk")); err == nil {
		idx = n
	}
	d := dataDisk(name, idx, "sd"+name)
	for i := range s.disks {
		if s.disks[i].name == name {
			s.disks[i] = d
			s.writeDisks()
			return
		}
	}
	// Insert after the last data disk, before the parity2/cache/flash sections.
	at := len(s.disks)
	for i, sec := range s.disks {
		if sec.vals["type"] == "Data" {
			at = i + 1
		}
	}
	s.disks = append(s.disks[:at], append([]section{d}, s.disks[at:]...)...)
	s.writeDisks()
}

// SetDisk sets one key of a Disk's section in disks.ini, adding the key if
// it is new.
func (s *Status) SetDisk(name, key, value string) {
	s.t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.disks {
		if s.disks[i].name == name {
			s.disks[i].set(key, value)
			s.writeDisks()
			return
		}
	}
	s.t.Fatalf("status: no disk %q", name)
}

// SetVar sets one key of var.ini, adding the key if it is new.
func (s *Status) SetVar(key, value string) {
	s.t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setVar(key, value)
	s.writeVars()
}

// StartParityCheck makes var.ini show a parity check running.
func (s *Status) StartParityCheck() {
	s.setVars("mdResyncAction", "check P", "mdResync", kib(parityKiB), "mdResyncPos", "123456",
		"mdResyncDt", "5", "mdResyncDb", "1048576")
}

// PauseParityCheck makes var.ini show a parity check in progress but paused.
func (s *Status) PauseParityCheck() {
	s.setVars("mdResyncAction", "check P", "mdResync", "0", "mdResyncPos", "123456",
		"mdResyncDt", "0", "mdResyncDb", "0")
}

// StartRebuild makes var.ini show disk being rebuilt ("recon D2" for disk2).
func (s *Status) StartRebuild(disk string) {
	s.setVars("mdResyncAction", "recon D"+strings.TrimPrefix(disk, "disk"),
		"mdResync", kib(parityKiB), "mdResyncPos", "123456", "mdResyncDt", "5", "mdResyncDb", "1048576")
}

// StartMover makes var.ini show the mover running.
func (s *Status) StartMover() { s.setVars("shareMoverActive", "yes") }

// StopArray makes var.ini show the array stopped.
func (s *Status) StopArray() { s.setVars("mdState", "STOPPED", "fsState", "Stopped") }

// Idle makes var.ini show a started array with no parity operation or mover.
func (s *Status) Idle() {
	s.setVars("mdState", "STARTED", "fsState", "Started", "mdResyncAction", "check P",
		"mdResync", "0", "mdResyncPos", "0", "mdResyncDt", "0", "mdResyncDb", "0",
		"shareMoverActive", "no")
}

func (s *Status) setVars(kv ...string) {
	s.t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i+1 < len(kv); i += 2 {
		s.setVar(kv[i], kv[i+1])
	}
	s.writeVars()
}

func (s *Status) setVar(key, value string) {
	if _, ok := s.vars[key]; !ok {
		s.order = append(s.order, key)
	}
	s.vars[key] = value
}

func (s *Status) writeDisks() {
	var b strings.Builder
	for _, sec := range s.disks {
		fmt.Fprintf(&b, "[%q]\n", sec.name)
		for _, k := range sec.keys {
			fmt.Fprintf(&b, "%s=%q\n", k, sec.vals[k])
		}
	}
	s.write("disks.ini", b.String())
}

func (s *Status) writeVars() {
	var b strings.Builder
	for _, k := range s.order {
		fmt.Fprintf(&b, "%s=%q\n", k, s.vars[k])
	}
	s.write("var.ini", b.String())
}

func (s *Status) write(name, content string) {
	s.t.Helper()
	tmp, err := os.CreateTemp(s.Dir, "."+name+".*")
	if err == nil {
		_, err = tmp.WriteString(content)
		if cerr := tmp.Close(); err == nil {
			err = cerr
		}
		if err == nil {
			err = os.Rename(tmp.Name(), filepath.Join(s.Dir, name))
		}
	}
	if err != nil {
		s.t.Fatalf("status: write %s: %v", name, err)
	}
}

func newSection(name string, kv ...string) section {
	sec := section{name: name, vals: map[string]string{}}
	sec.set("name", name)
	for i := 0; i+1 < len(kv); i += 2 {
		sec.set(kv[i], kv[i+1])
	}
	return sec
}

func (sec *section) set(key, value string) {
	if _, ok := sec.vals[key]; !ok {
		sec.keys = append(sec.keys, key)
	}
	sec.vals[key] = value
}

func dataDisk(name string, idx int, device string) section {
	return newSection(name, "idx", strconv.Itoa(idx), "device", device, "size", kib(dataKiB),
		"status", "DISK_OK", "type", "Data", "color", "green-on", "rotational", "1",
		"transport", "ata", "fsType", "xfs", "fsStatus", "Mounted",
		"fsMountpoint", "/mnt/"+name, "fsSize", kib(dataKiB), "fsUsed", "0",
		"fsFree", kib(dataKiB), "deviceSb", "md"+strconv.Itoa(idx)+"p1")
}

func kib(n int64) string { return strconv.FormatInt(n, 10) }

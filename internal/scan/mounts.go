package scan

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DefaultMountInfo is where Linux lists the mounts the process sees.
const DefaultMountInfo = "/proc/self/mountinfo"

// mountPoints are the folders where a filesystem is mounted.
type mountPoints map[string]bool

func (m mountPoints) has(path string) bool { return m[path] }

// readMountPoints reads every mount point from a mountinfo file
// (proc_pid_mountinfo(5)). It is read afresh for every Scan: mounts change.
func readMountPoints(file string) (mountPoints, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("mount points: %w", err)
	}
	m := mountPoints{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		// 36 35 98:0 /mnt1 /mnt/parent rw,noatime master:1 - ext3 /dev/root rw
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 {
			continue
		}
		m[unescapeMountPath(fields[4])] = true
	}
	return m, sc.Err()
}

// unescapeMountPath undoes mountinfo's octal escapes of space, tab, newline
// and backslash ("\040" is a space).
func unescapeMountPath(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

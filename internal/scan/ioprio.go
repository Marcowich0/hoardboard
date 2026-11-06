package scan

import (
	"golang.org/x/sys/unix"
)

// Linux's I/O priority (ioprio_set(2)); x/sys has no wrapper.
const (
	ioprioWhoProcess = 1 // with who=0: the calling thread
	ioprioClassIdle  = 3
	ioprioClassShift = 13
)

// beIdle gives the calling OS thread the idle I/O class, so that its disk
// reads wait for everyone else's, and the lowest CPU priority. Both are per
// thread on Linux and need no capability. The caller must have locked its
// goroutine to the thread (runtime.LockOSThread) and never unlock it, so
// that the thread ends with the goroutine and the priority never reaches
// other work.
func beIdle() error {
	if _, _, e := unix.Syscall(unix.SYS_IOPRIO_SET, ioprioWhoProcess, 0, ioprioClassIdle<<ioprioClassShift); e != 0 {
		return e
	}
	return unix.Setpriority(unix.PRIO_PROCESS, 0, 19)
}

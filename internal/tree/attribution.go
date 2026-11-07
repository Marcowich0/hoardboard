package tree

import "strings"

// InTorrents reports whether path is a Folder named torrents or below one.
// Names are matched exactly, like Linux Folder names; my-torrents is unrelated.
func InTorrents(path string) bool {
	for name := range strings.SplitSeq(path, "/") {
		if name == "torrents" {
			return true
		}
	}
	return false
}

// CountingPaths is the Folders whose hardlink names count toward Size.
// If a Scan saw any non-torrents name, torrents names only seed that file:
// they remain visible, but their bytes count only outside torrents.
// Without a non-torrents name, every observed name counts as usual.
// This also applies to History, whose link sets preserve all observed names.
func CountingPaths(paths []string) []string {
	var outside []string
	for _, path := range paths {
		if !InTorrents(path) {
			outside = append(outside, path)
		}
	}
	if len(outside) == 0 {
		return paths
	}
	return outside
}

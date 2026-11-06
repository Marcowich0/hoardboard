package inventory

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// section is one `["name"]` block of an emhttpd status file: its keys and
// values, unquoted.
type section struct {
	name string
	vals map[string]string
}

// readINI reads a status file as emhttpd writes it: optional `["name"]`
// section headers, then `key="value"` lines. Lines before the first header
// (all of var.ini, which has none) go into a section named "". Blank and
// unknown lines are ignored. The file is read afresh every time: emhttpd
// rewrites it as the array changes.
func readINI(path string) ([]section, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	secs := []section{{vals: map[string]string{}}}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.Trim(line[1:len(line)-1], `"`)
			secs = append(secs, section{name: name, vals: map[string]string{}})
			continue
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		val, err := strconv.Unquote(raw)
		if err != nil {
			val = strings.Trim(raw, `"`)
		}
		secs[len(secs)-1].vals[strings.TrimSpace(key)] = val
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return secs, nil
}

// int reads key as a whole number, 0 if it is missing or not a number.
func (s section) int(key string) int64 {
	n, _ := strconv.ParseInt(s.vals[key], 10, 64)
	return n
}

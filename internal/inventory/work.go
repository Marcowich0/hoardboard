package inventory

import "strings"

// ArrayWork says what heavy work Unraid is doing on the array right now, as
// the reason a Scan should stay out of its way, or "" if there is none. It
// reads var.ini afresh, the way Unraid's own GUI decides:
//
//   - fsState other than "Started": the array is not started, so nothing is
//     mounted to scan;
//   - mdResyncPos > 0: a parity operation is in progress, running or paused
//     (mdResync == 0); its kind is the first word of mdResyncAction: check,
//     recon (a parity sync, or a rebuild of a data disk) or clear;
//   - shareMoverActive "yes": the mover runs.
//
// A var.ini without fsState (or none at all, outside Unraid) counts as a
// started array.
func (inv *Inventory) ArrayWork() (string, error) {
	vars, err := inv.vars()
	if err != nil {
		return "", err
	}
	if st := vars.vals["fsState"]; st != "" && st != "Started" {
		return "Array is not started", nil
	}
	if vars.int("mdResyncPos") > 0 {
		action := strings.Fields(vars.vals["mdResyncAction"])
		what := "Array operation"
		if len(action) > 0 {
			switch action[0] {
			case "check":
				what = "Parity check"
			case "recon":
				what = "Parity sync"
				for _, target := range action[1:] {
					if strings.HasPrefix(target, "D") {
						what = "Disk rebuild"
					}
				}
			case "clear":
				what = "Disk clear"
			}
		}
		if vars.int("mdResync") == 0 {
			return what + " paused", nil
		}
		return what + " in progress", nil
	}
	if vars.vals["shareMoverActive"] == "yes" {
		return "Mover is running", nil
	}
	return "", nil
}

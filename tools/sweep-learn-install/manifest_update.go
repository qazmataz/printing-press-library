// manifest_update.go updates the single printing_press_version field
// in a CLI's .printing-press.json. All other manifest fields are
// preserved verbatim — the rewrite pass uses a regex against the
// quoted field so JSON-level ordering, comments, and extra fields
// land back on disk untouched.
//
// The sweep deliberately does not write any other field of the
// manifest, including run_id or printer. Those values were set when
// the CLI was generated and reflect that artifact's provenance.

package main

import (
	"fmt"
	"regexp"
)

var printingPressVersionRe = regexp.MustCompile(`("printing_press_version"\s*:\s*")[^"]*(")`)

// updatePrintingPressVersion replaces the value of
// "printing_press_version" with the target. Returns the new bytes,
// a changed boolean, and any error. The caller decides whether to
// write the result.
//
// If the field is absent, the function returns the input unchanged
// without error — the sweep treats this as "manifest predates the
// learn loop; nothing to bump." Manifests with a non-string value
// for the field are also left untouched (the regex doesn't match).
func updatePrintingPressVersion(data []byte, target string) ([]byte, bool, error) {
	if !printingPressVersionRe.Match(data) {
		return data, false, nil
	}
	replaced := false
	out := printingPressVersionRe.ReplaceAllFunc(data, func(match []byte) []byte {
		sub := printingPressVersionRe.FindSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		newLine := fmt.Sprintf("%s%s%s", sub[1], target, sub[2])
		if string(newLine) != string(match) {
			replaced = true
		}
		return []byte(newLine)
	})
	return out, replaced, nil
}

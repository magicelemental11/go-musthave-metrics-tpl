// Package buildinfo prints the version, date and commit a binary was built from.
package buildinfo

import (
	"cmp"
	"fmt"
	"io"
)

const unknown = "N/A"

// Fprint writes the build version, date and commit to w, using N/A for empty values.
func Fprint(w io.Writer, version, date, commit string) {
	fmt.Fprintf(w, "Build version: %s\n", cmp.Or(version, unknown))
	fmt.Fprintf(w, "Build date: %s\n", cmp.Or(date, unknown))
	fmt.Fprintf(w, "Build commit: %s\n", cmp.Or(commit, unknown))
}

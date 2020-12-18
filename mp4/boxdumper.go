package mp4

import (
	"fmt"
	"io"
)

// boxDumper - dump box name and size. Allow for more with write.
type boxDumper struct {
	w      io.Writer
	indent string
	box    Box
	err    error
}

// newBoxDumper - make a boxDumper with max level on what to write
// set Version to -1 if not present
func newBoxDumper(w io.Writer, indent string, b Box, version int) *boxDumper {
	bd := boxDumper{w, indent, b, nil}
	if version < 0 {
		bd.write("%s size=%d", b.Type(), b.Size())
	} else {
		bd.write("%s size=%d version=%d", b.Type(), b.Size(), version)
	}

	return &bd
}

// write - write formated objecds if level <= bd.level
func (b boxDumper) write(format string, p ...interface{}) {
	if b.err != nil {
		return
	}
	_, b.err = fmt.Fprintf(b.w, "%s", b.indent)
	if b.err != nil {
		return
	}
	_, b.err = fmt.Fprintf(b.w, format+"\n", p...)
}

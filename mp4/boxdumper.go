package mp4

import (
	"fmt"
	"io"
	"strconv"
	"strings"
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
		bd.write("[%s] size=%d", b.Type(), b.Size())
	} else {
		bd.write("[%s] size=%d version=%d", b.Type(), b.Size(), version)
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

// getDumpLevel - get dump level for box, or from all
func getDumpLevel(b Box, specificBoxLevels string) (level int) {
	if len(specificBoxLevels) == 0 {
		return level
	}
	boxesLevels := strings.Split(specificBoxLevels, ",")
	boxType := b.Type()
	var err error
	for _, bl := range boxesLevels {
		splitPos := strings.Index(bl, ":")
		if splitPos < 1 {
			continue
		}
		bt := bl[:splitPos]
		nr := bl[splitPos+1:]
		if bt == boxType {
			level, err = strconv.Atoi(nr)
			if err != nil {
				level = 0
			}
			return level
		} else if bt == "all" {
			level, err = strconv.Atoi(nr)
			if err != nil {
				level = 0
			}
		}
	}
	return level
}

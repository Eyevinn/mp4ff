package mp4

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

type boxLike interface {
	Type() string
	Size() uint64
	Info(w io.Writer, specificBoxLevels, indent, indentStep string) error
}

// infoDumper - dump box name and size. Allow for more with write.
type infoDumper struct {
	w      io.Writer
	indent string
	box    boxLike
	err    error
}

// newInfoDumper - make an infoDumper with indent
// set Version to -1 if not present for box
// set Version to -2 for sample group entries
func newInfoDumper(w io.Writer, indent string, b boxLike, version int, flags uint32) *infoDumper {
	bd := infoDumper{w, indent, b, nil}
	if version == -1 {
		bd.write("[%s] size=%d", b.Type(), b.Size())
	} else if version >= 0 {
		bd.write("[%s] size=%d version=%d flags=%06x", b.Type(), b.Size(), version, flags)
	} else { // version = -2
		bd.write("GroupingType %q size=%d", b.Type(), b.Size())
	}
	return &bd
}

// write - write formated objecds if level <= bd.level
func (b infoDumper) write(format string, p ...interface{}) {
	if b.err != nil {
		return
	}
	_, b.err = fmt.Fprintf(b.w, "%s", b.indent)
	if b.err != nil {
		return
	}
	_, b.err = fmt.Fprintf(b.w, format+"\n", p...)
}

// getInfoLevel - get info level for specific boxLike, or from all
func getInfoLevel(b boxLike, specificBoxLevels string) (level int) {
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

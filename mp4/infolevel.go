package mp4

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

// maxInfoLevel is higher than any level at which an Info method writes more details.
const maxInfoLevel = 9

// infoIndentStep is the indentation per box level of InfoLines and MoreInfoLevel.
const infoIndentStep = "  "

// errStopInfo stops an Info method that writes to a lineWriter.
var errStopInfo = errors.New("stop info")

// lineWriter calls fn with each line that an Info method writes about the box itself, up to the first line of a
// child box, and stops the Info method when fn returns false.
type lineWriter struct {
	fn      func(line string) bool
	n       int // the number of lines
	buf     []byte
	stopped bool
}

func (w *lineWriter) Write(p []byte) (int, error) {
	if w.stopped {
		return 0, errStopInfo
	}
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			return len(p), nil
		}
		line := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		if (w.n > 0 && strings.HasPrefix(line, infoIndentStep+"[")) || !w.fn(line) {
			w.stopped = true
			return len(p), errStopInfo
		}
		w.n++
	}
}

// boxInfoLines calls fn with each line that the Info method of box writes about the box itself, until fn returns
// false.
func boxInfoLines(box Box, specificBoxLevels string, fn func(line string) bool) {
	w := &lineWriter{fn: fn}
	_ = box.Info(w, specificBoxLevels, "", infoIndentStep)
	if len(w.buf) > 0 && !w.stopped {
		w.fn(string(w.buf))
	}
}

// InfoLines returns the lines that the Info method of box writes about the box itself, without the lines of its
// child boxes, with the levels of detail in specificBoxLevels.
func InfoLines(box Box, specificBoxLevels string) []string {
	var lines []string
	boxInfoLines(box, specificBoxLevels, func(line string) bool {
		lines = append(lines, line)
		return true
	})
	return lines
}

// MoreInfoLevel returns the lowest level above the one that specificBoxLevels sets for box, at which the Info method
// of box writes more or other lines about the box itself, or 0 if there is no such level. The lines of child boxes
// do not count. Setting the level for the box type, as with "pssh:2" in specificBoxLevels or the -l option of
// mp4ff-info, shows those details.
//
// The details of parts of a box with a type of their own, such as the sample group entries in sgpd with their
// grouping type, are not included.
func MoreInfoLevel(box Box, specificBoxLevels string) int {
	level := getInfoLevel(box, specificBoxLevels)
	if level >= maxInfoLevel {
		return 0
	}
	current := InfoLines(box, specificBoxLevels)
	// differs tells if the lines at level l differ from the current ones. It stops at the first difference, so
	// that a long list of samples is not written in full.
	differs := func(l int) bool {
		levels := fmt.Sprintf("%s:%d", box.Type(), l)
		if specificBoxLevels != "" {
			levels += "," + specificBoxLevels
		}
		n, differ := 0, false
		boxInfoLines(box, levels, func(line string) bool {
			if n == len(current) || line != current[n] {
				differ = true
				return false
			}
			n++
			return true
		})
		return differ || n < len(current)
	}
	// Higher levels add details, so if there are none at maxInfoLevel, there are none at any level.
	if !differs(maxInfoLevel) {
		return 0
	}
	for l := level + 1; l < maxInfoLevel; l++ {
		if differs(l) {
			return l
		}
	}
	return maxInfoLevel
}

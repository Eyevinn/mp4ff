package filter

import (
	"io"

	"github.com/jfbus/mp4"
)

type noopFilter struct{}

// Noop returns a filter that does nothing
func Noop() Filter {
	return &noopFilter{}
}

func (f *noopFilter) FilterMoov(m *mp4.MoovBox) error {
	return nil
}

func (f *noopFilter) FilterMdat(w io.Writer, m *mp4.MdatBox) error {
	err := mp4.EncodeHeader(m, w)
	if err == nil {
		_, err = io.Copy(w, m.Reader())
	}
	return err
}

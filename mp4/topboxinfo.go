package mp4

import (
	"fmt"
	"io"
)

type TopBoxInfo struct {
	Name     string
	Size     uint64
	StartPos uint64
}

// GetTopBoxInfoList - get top boxes until stopBox or end of file
func GetTopBoxInfoList(rs io.ReadSeeker, stopBox string) ([]TopBoxInfo, error) {
	var pos uint64 = 0
	var topBoxList []TopBoxInfo

	for {
		h, err := decodeHeader(rs)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.name == stopBox {
			break
		}
		topBoxList = append(topBoxList, TopBoxInfo{h.name, h.size, pos})
		nextBoxStart := pos + h.size
		ipos, err := rs.Seek(int64(nextBoxStart), io.SeekStart)
		if err != nil {
			return nil, err
		}
		pos = uint64(ipos)
		if pos != nextBoxStart {
			return nil, fmt.Errorf("seeked pos %d != next box start %d", pos, nextBoxStart)
		}
	}

	return topBoxList, nil
}

package mp4

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"io"
)

// DecryptBytesCTR - decrypt or encrypt sample using CTR mode, provided key, iv and sumsamplePattern
func DecryptBytesCTR(data []byte, key []byte, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewCTR(block, iv)

	inBuf := bytes.NewBuffer(data)
	outBuf := bytes.Buffer{}

	writer := cipher.StreamWriter{S: stream, W: &outBuf}
	_, err = io.Copy(writer, inBuf)
	if err != nil {
		return nil, err
	}
	return outBuf.Bytes(), nil
}

// DecryptSampleCenc - decrypt cenc-mode encrypted sample
func DecryptSampleCenc(sample []byte, key []byte, iv []byte, subSamplePatterns []SubSamplePattern) ([]byte, error) {
	decSample := make([]byte, 0, len(sample))
	if len(subSamplePatterns) != 0 {
		var pos uint32 = 0
		for j := 0; j < len(subSamplePatterns); j++ {
			ss := subSamplePatterns[j]
			nrClear := uint32(ss.BytesOfClearData)
			nrEnc := ss.BytesOfProtectedData
			decSample = append(decSample, sample[pos:pos+nrClear]...)
			pos += nrClear
			cryptOut, err := DecryptBytesCTR(sample[pos:pos+nrEnc], key, iv)
			if err != nil {
				return nil, err
			}
			decSample = append(decSample, cryptOut...)
			pos += nrEnc
		}
	} else {
		cryptOut, err := DecryptBytesCTR(sample, key, iv)
		if err != nil {
			return nil, err
		}
		decSample = append(decSample, cryptOut...)
	}
	return decSample, nil
}

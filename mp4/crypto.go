package mp4

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"io"
)

// DecryptSampleCTR - decrypt or encrypt sample using CTR mode and provided key and iv
func DecryptSampleCTR(sample []byte, key []byte, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewCTR(block, iv)

	inBuf := bytes.NewBuffer(sample)
	outBuf := bytes.Buffer{}

	writer := cipher.StreamWriter{S: stream, W: &outBuf}
	_, err = io.Copy(writer, inBuf)
	if err != nil {
		return nil, err
	}
	return outBuf.Bytes(), nil
}

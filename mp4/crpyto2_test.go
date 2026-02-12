package mp4

import (
	"bytes"
	"embed"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

//go:embed test_data/*.m4s  test_data/*.m4v
var files embed.FS

func Test_Encrypt(t *testing.T) {

	// key := make([]byte, 32)
	// iv16 := make([]byte, 16)
	key := []byte("1fTOHpfh0BP7zqVKgc4hdxqAGfI3X984")
	iv16 := []byte("0123456789abcdef")
	keyID := GenerateKeyID()
	keyset1 := &ProtectKey{
		StreamType: "video",
		Resolution: "1920x1080",
		TrackId:    2,
		Key:        key,
		Iv:         iv16,
		Kid:        keyID,
	}
	keyset2 := &ProtectKey{
		StreamType: "audio",
		Resolution: "1920x1080",
		TrackId:    1,
		Key:        []byte("1fTOHpfh0BP7zqVKgc4hdxqAGfI3X982"),
		Iv:         []byte("0123456789abcded"),
		Kid:        GenerateKeyID(),
	}
	keylist := []*ProtectKey{keyset1, keyset2}
	enc := NewMP4Encryptor(keylist)
	if err := os.MkdirAll(filepath.Join(".", "test_data", "encrypted"), 0777); err != nil {
		t.Fatalf("failed to create test data directory: %v", err)
	}
	f, err := files.ReadFile(filepath.Join("test_data", "stream_moov_1.m4s"))
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}

	enc.AddPSSH(UUIDWidevine, []byte("AAAAoHBzc2gAAAAA7e+LqXnWSs6jyCfc1R0h7QAAAIASEHpGiWcpnsLv9vkqUUzS09AaDGlua2FlbnR3b3JrcyJYcmNZMFc3ZnN3TEk2Z1Y2Z1hOVkdSbkJ5eGd3d2tOUC10cnVpcVpTYWllbGhOTlZGckRac3YwR25wY1MtTEg0X1VyM1B0V0J6cExfRlJkdUk4eldCU1E9PUjzxombBg=="), keyID, GenerateKeyID())

	encryptedInit, err := enc.InitProtect(bytes.NewBuffer(f))
	if err != nil {
		t.Fatalf("failed to init protect: %v", err)
	}
	if err := os.WriteFile(filepath.Join("test_data", "encrypted", "stream_moov_1_encrpyted.m4s"), encryptedInit, 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

}

func GenerateKeyID() UUID {
	v, _ := uuid.NewV7()
	cv := ([16]byte)(v)
	return cv[:]
}

type MP4Encryptor struct {
	keys []*ProtectKey
	// psshs
	psshs []*PsshBox
	// initProtect
	initProtects []*InitProtectData
}

func NewMP4Encryptor(keys []*ProtectKey) *MP4Encryptor {
	return &MP4Encryptor{
		keys:         keys,
		psshs:        make([]*PsshBox, 0),
		initProtects: nil,
	}
}

func (m *MP4Encryptor) AddPSSH(hexSystemId string, data []byte, kIds ...UUID) {
	cleaned := strings.ReplaceAll(hexSystemId, "-", "")
	uuidBytes, err := hex.DecodeString(cleaned)
	systemId := []byte(uuidBytes)
	if err != nil {
		panic(err)
	}

	version := byte(0)
	if len(kIds) > 0 {
		version = byte(1)
	}

	pssh := &PsshBox{
		Version:  version,
		Flags:    0, //default
		SystemID: systemId,
		KIDs:     kIds,
		Data:     data,
	}
	m.psshs = append(m.psshs, pssh)
}

func (m *MP4Encryptor) InitProtect(initSeg io.Reader) ([]byte, error) {

	f, err := DecodeFile(initSeg)
	if err != nil {
		return nil, fmt.Errorf("mp4.DecodeFile: %w", err)
	}

	_, err = InitMultitrackProtect(f.Init, "cbcs", m.keys, m.psshs) // psshs added into init
	if err != nil {
		return nil, fmt.Errorf("mp4.InitProtect: %w", err)
	}

	buf := bytes.NewBuffer(nil)
	if err := f.Encode(buf); err != nil {
		return nil, fmt.Errorf("mp4.InitProtect encode: %w", err)
	}

	return buf.Bytes(), nil
}

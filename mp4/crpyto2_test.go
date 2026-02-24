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

//go:embed test_data
var files embed.FS

func Test_Encrypt(t *testing.T) {
	key := []byte("1fTOHpfh0BP7zqVKgc4hdxqAGfI3X984")
	iv16 := []byte("0123456789abcdef")
	keyID := GenerateKeyID()
	keyset1 := &ProtectKey{
		StreamType: "video",
		Resolution: "1920x1080",
		TrackId:    1,
		Key:        key,
		Iv:         iv16,
		Kid:        keyID,
	}
	keyset2 := &ProtectKey{
		StreamType: "audio",
		Resolution: "1920x1080",
		TrackId:    2,
		Key:        []byte("1fTOHpfh0BP7zqVKgc4hdxqAGfI3X982"),
		Iv:         []byte("0123456789abcded"),
		Kid:        GenerateKeyID(),
	}
	keylist := []*ProtectKey{keyset1, keyset2}
	enc := NewMP4Encryptor(keylist)
	if err := os.MkdirAll(filepath.Join(".", "test_data", "encrypted"), 0777); err != nil {
		t.Fatalf("failed to create test data directory: %v", err)
	}
	f, err := files.ReadFile(filepath.Join("test_data", "init.mp4"))
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}

	enc.AddPSSH(UUIDWidevine, []byte("AAAAoHBzc2gAAAAA7e+LqXnWSs6jyCfc1R0h7QAAAIASEHpGiWcpnsLv9vkqUUzS09AaDGlua2FlbnR3b3JrcyJYcmNZMFc3ZnN3TEk2Z1Y2Z1hOVkdSbkJ5eGd3d2tOUC10cnVpcVpTYWllbGhOTlZGckRac3YwR25wY1MtTEg0X1VyM1B0V0J6cExfRlJkdUk4eldCU1E9PUjzxombBg=="), keyID, GenerateKeyID())

	encryptedInit, err := enc.InitProtect(bytes.NewBuffer(f))
	if err != nil {
		t.Fatalf("failed to init protect: %v", err)
	}
	if err := os.WriteFile(filepath.Join("test_data", "encrypted", "encrypted_init.mp4"), encryptedInit, 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	for _, seg := range []string{"segment_000", "segment_001", "segment_002"} {
		func() {
			f, err := files.ReadFile(filepath.Join("test_data", fmt.Sprintf("%s.m4s", seg)))
			if err != nil {
				t.Fatalf("failed to open file: %v", err)
			}

			d, err := enc.Encrypt(bytes.NewBuffer(f))
			if err != nil {
				t.Fatalf("failed to encrypt: %v", err)
			}

			if err := os.WriteFile(filepath.Join("test_data", "encrypted", fmt.Sprintf("encrypted_%s.m4s", seg)), d, 0600); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}
		}()
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
	initProtects map[uint32]*InitProtectData
}

func NewMP4Encryptor(keys []*ProtectKey) *MP4Encryptor {
	return &MP4Encryptor{
		keys:         keys,
		psshs:        make([]*PsshBox, 0),
		initProtects: make(map[uint32]*InitProtectData),
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

	ipd, err := InitMultitrackProtect(f.Init, "cbcs", m.keys, m.psshs) // psshs added into init
	if err != nil {
		return nil, fmt.Errorf("mp4.InitProtect: %w", err)
	}
	m.initProtects = ipd

	buf := bytes.NewBuffer(nil)
	if err := f.Encode(buf); err != nil {
		return nil, fmt.Errorf("mp4.InitProtect encode: %w", err)
	}

	return buf.Bytes(), nil
}

func (m *MP4Encryptor) Encrypt(segData io.Reader) ([]byte, error) {
	seg, err := DecodeFile(segData)
	if err != nil {
		return nil, fmt.Errorf("mp4.DecodeFile: %w", err)
	}

	for _, s := range seg.Segments {
		for _, f := range s.Fragments {
			//for _, pssh := range m.psshs {
			//	if err = f.Moof.AddChild(pssh); err != nil {
			//		return nil, fmt.Errorf("mp4.AddPsshInFragment: %w", err)
			//	}
			//}

			if err = EncryptMultitrackFragment(f, m.keys, m.initProtects); err != nil {
				return nil, fmt.Errorf("mp4.EncryptFragment: %w", err)
			}
		}
	}

	buf := bytes.NewBuffer(nil)
	if err := seg.Encode(buf); err != nil {
		return nil, fmt.Errorf("mp4.Encode: %w", err)
	}

	return buf.Bytes(), nil
}

type MP4Decryptor struct {
	keys []*ProtectKey
	// psshs
	psshs []*PsshBox
	// initProtect
	initProtects map[uint32]*InitProtectData

	decryptedInfo *DecryptInfo
}

func NewMP4Decryptor(keys []*ProtectKey) *MP4Decryptor {
	return &MP4Decryptor{
		keys:         keys,
		psshs:        make([]*PsshBox, 0),
		initProtects: make(map[uint32]*InitProtectData),
	}
}

func Test_Decrypt(t *testing.T) {
	key := []byte("1fTOHpfh0BP7zqVKgc4hdxqAGfI3X984")
	iv16 := []byte("0123456789abcdef")
	keyID := GenerateKeyID()
	keyset1 := &ProtectKey{
		StreamType: "video",
		Resolution: "1920x1080",
		TrackId:    1,
		Key:        key,
		Iv:         iv16,
		Kid:        keyID,
	}
	keyset2 := &ProtectKey{
		StreamType: "audio",
		Resolution: "1920x1080",
		TrackId:    2,
		Key:        []byte("1fTOHpfh0BP7zqVKgc4hdxqAGfI3X982"),
		Iv:         []byte("0123456789abcded"),
		Kid:        GenerateKeyID(),
	}
	keylist := []*ProtectKey{keyset1, keyset2}

	dec := NewMP4Decryptor(keylist)
	if err := os.MkdirAll(filepath.Join(".", "test_data", "decrypted"), 0777); err != nil {
		t.Fatalf("failed to create test data directory: %v", err)
	}
	f, err := files.ReadFile(filepath.Join("test_data", "encrypted", "encrypted_init.mp4"))
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	decryptedInit, err := dec.DecryptInit(bytes.NewBuffer(f))
	if err != nil {
		t.Fatalf("failed to init protect: %v", err)
	}

	rawInit, err := files.ReadFile(filepath.Join("test_data", "init.mp4"))
	if err != nil {
		t.Fatalf("failed to get raw seg: %v", err)
	}

	if !bytes.Equal(rawInit, decryptedInit) {
		t.Errorf("segment not equal after encryption+decryption")
	}

	if err := os.WriteFile(filepath.Join("test_data", "decrypted", "decrypted_init.mp4"), decryptedInit, 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	for _, seg := range []string{"segment_000", "segment_001", "segment_002"} {
		func() {
			f, err = files.ReadFile(filepath.Join("test_data", "encrypted", fmt.Sprintf("encrypted_%s.m4s", seg)))
			if err != nil {
				t.Fatalf("failed to open file: %v", err)
			}

			d, err := dec.Decrypt(bytes.NewBuffer(f))
			if err != nil {
				t.Fatalf("failed to decrypt: %v", err)
			}
			rawSeg, err := files.ReadFile(filepath.Join("test_data", fmt.Sprintf("%s.m4s", seg)))
			if err != nil {
				t.Fatalf("failed to get raw seg: %v", err)
			}

			if !bytes.Equal(rawSeg, d) {
				t.Errorf("segment not equal after encryption+decryption")
			}

			if err := os.WriteFile(filepath.Join("test_data", "decrypted", fmt.Sprintf("decrypted_%s.m4s", seg)), d, 0600); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}
		}()
	}
}

func (m *MP4Decryptor) DecryptInit(initSeg io.Reader) ([]byte, error) {
	f, err := DecodeFile(initSeg)
	if err != nil {
		return nil, fmt.Errorf("mp4.DecodeFile: %w", err)
	}

	decinfo, err := DecryptInit(f.Init)
	if err != nil {
		return nil, fmt.Errorf("mp4.DecryptInit: %w", err)
	}
	m.decryptedInfo = &decinfo

	buf := bytes.NewBuffer(nil)
	if err := f.Encode(buf); err != nil {
		return nil, fmt.Errorf("mp4.DecryptedInit encode: %w", err)
	}

	return buf.Bytes(), nil
}

func (m *MP4Decryptor) Decrypt(segData io.Reader) ([]byte, error) {
	if m.decryptedInfo == nil {
		return nil, fmt.Errorf("DecryptedInit not called")
	}

	seg, err := DecodeFile(segData)
	if err != nil {
		return nil, fmt.Errorf("mp4.DecodeFile: %w", err)
	}

	for _, s := range seg.Segments {
		if err = DecryptMultiTrackSegment(s, *m.decryptedInfo, m.keys); err != nil {
			return nil, err
		}
	}

	buf := bytes.NewBuffer(nil)
	if err := seg.Encode(buf); err != nil {
		return nil, fmt.Errorf("mp4.Encode: %w", err)
	}

	return buf.Bytes(), nil
}

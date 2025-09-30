package main

import (
	"encoding/hex"
	"fmt"

	"github.com/Eyevinn/mp4ff/mp4"
)

type EncryptConfig struct {
	Key    []byte
	KeyID  []byte
	IV     []byte
	Scheme string
}

type StreamEncryptor struct {
	config        EncryptConfig
	ipd           *mp4.InitProtectData
	fragNum       uint32
	encryptedInit *mp4.InitSegment
}

func NewStreamEncryptor(init *mp4.InitSegment, config EncryptConfig) (*StreamEncryptor, error) {
	if len(config.IV) != 16 && len(config.IV) != 8 {
		return nil, fmt.Errorf("IV must be 8 or 16 bytes")
	}
	if len(config.Key) != 16 {
		return nil, fmt.Errorf("key must be 16 bytes")
	}
	if len(config.KeyID) != 16 {
		return nil, fmt.Errorf("keyID must be 16 bytes")
	}

	kidHex := hex.EncodeToString(config.KeyID)
	kidUUID, err := mp4.NewUUIDFromString(kidHex)
	if err != nil {
		return nil, fmt.Errorf("invalid key ID: %w", err)
	}

	ipd, err := mp4.InitProtect(init, config.Key, config.IV, config.Scheme, kidUUID, nil)
	if err != nil {
		return nil, fmt.Errorf("init protect: %w", err)
	}

	return &StreamEncryptor{
		config:        config,
		ipd:           ipd,
		encryptedInit: init,
	}, nil
}

func (se *StreamEncryptor) GetEncryptedInit() *mp4.InitSegment {
	return se.encryptedInit
}

func (se *StreamEncryptor) EncryptFragment(frag *mp4.Fragment) error {
	se.fragNum++

	iv := se.deriveIV(se.fragNum)

	err := mp4.EncryptFragment(frag, se.config.Key, iv, se.ipd)
	if err != nil {
		return fmt.Errorf("encrypt fragment %d: %w", se.fragNum, err)
	}

	return nil
}

func (se *StreamEncryptor) deriveIV(fragNum uint32) []byte {
	iv := make([]byte, 16)
	copy(iv, se.config.IV)

	for i := 15; i >= 12; i-- {
		carry := fragNum & 0xFF
		sum := uint32(iv[i]) + carry
		iv[i] = byte(sum & 0xFF)
		fragNum = fragNum >> 8
		if fragNum == 0 {
			break
		}
	}

	return iv
}

func ParseHexKey(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

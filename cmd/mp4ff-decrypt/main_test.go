package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestNonRunningOptionCases(t *testing.T) {
	infile := "../../mp4/testdata/cbcs_audio.mp4"
	nonEncryptedFile := "../../mp4/testdata/prog_8s_dec_dashinit.mp4"
	key := "00112233445566778899aabbccddeeff"
	badKey := "00112233445566778899aabbccddeefx"
	tmpDir := t.TempDir()
	outFile := path.Join(tmpDir, "outfile.mp4")
	cases := []struct {
		desc string
		args []string
		err  bool
	}{
		{desc: "no args", args: []string{"mp4ff-decrypt"}, err: true},
		{desc: "unknown args", args: []string{"mp4ff-decrypt", "-x"}, err: true},
		{desc: "no outfile", args: []string{"mp4ff-decrypt", "infile.mp4"}, err: true},
		{desc: "no key", args: []string{"mp4ff-decrypt", "infile.mp4", outFile}, err: true},
		{desc: "non-existing infile", args: []string{"mp4ff-decrypt", "-key", key, "infile.mp4", outFile}, err: true},
		{desc: "non-existing initfile", args: []string{"mp4ff-decrypt", "-init", "init.mp4", "-key", key, infile, outFile}, err: true},
		{desc: "bad infile", args: []string{"mp4ff-decrypt", "-key", key, "main.go", outFile}, err: true},
		{desc: "short key", args: []string{"mp4ff-decrypt", "-key", "ab", infile, outFile}, err: true},
		{desc: "bad key", args: []string{"mp4ff-decrypt", "-key", badKey, infile, outFile}, err: true},
		{desc: "non-encrypted file", args: []string{"mp4ff-decrypt", "-key", key, nonEncryptedFile, outFile}, err: false},
		{desc: "version", args: []string{"mp4ff-decrypt", "-version"}, err: false},
		{desc: "help", args: []string{"mp4ff-decrypt", "-h"}, err: false},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			err := run(c.args)
			if c.err && err == nil {
				t.Error("expected error but got nil")
			}
			if !c.err && err != nil {
				t.Errorf("unexpected error: %s", err)
			}
		})
	}
}

func TestDecodeFiles(t *testing.T) {
	testCases := []struct {
		desc            string
		initFile        string
		inFile          string
		expectedOutFile string
		keyHexOrBase64  string
	}{
		{
			desc:            "cenc",
			inFile:          "../../mp4/testdata/prog_8s_enc_dashinit.mp4",
			expectedOutFile: "../../mp4/testdata/prog_8s_dec_dashinit.mp4",
			keyHexOrBase64:  "63cb5f7184dd4b689a5c5ff11ee6a328",
		},
		{
			desc:            "cenc with base64 key",
			inFile:          "../../mp4/testdata/prog_8s_enc_dashinit.mp4",
			expectedOutFile: "../../mp4/testdata/prog_8s_dec_dashinit.mp4",
			keyHexOrBase64:  "Y8tfcYTdS2iaXF/xHuajKA==",
		},
		{
			desc:            "cbcs",
			inFile:          "../../mp4/testdata/cbcs.mp4",
			expectedOutFile: "../../mp4/testdata/cbcsdec.mp4",
			keyHexOrBase64:  "22bdb0063805260307ee5045c0f3835a",
		},
		{
			desc:            "cbcs audio",
			inFile:          "../../mp4/testdata/cbcs_audio.mp4",
			expectedOutFile: "../../mp4/testdata/cbcs_audiodec.mp4",
			keyHexOrBase64:  "5ffd93861fa776e96cccd934898fc1c8",
		},
		{
			desc:            "PIFF audio",
			initFile:        "testdata/PIFF/audio/init.mp4",
			inFile:          "testdata/PIFF/audio/segment-1.0001.m4s",
			expectedOutFile: "testdata/PIFF/audio/segment-1.0001_dec.m4s",
			keyHexOrBase64:  "602a9289bfb9b1995b75ac63f123fc86",
		},
		{
			desc:            "PIFF video",
			inFile:          "testdata/PIFF/video/complseg-1.0001.mp4",
			expectedOutFile: "testdata/PIFF/video/complseg-1.0001_dec.mp4",
			keyHexOrBase64:  "602a9289bfb9b1995b75ac63f123fc86",
		},
	}
	tmpDir := t.TempDir()
	for nr, c := range testCases {
		t.Run(c.desc, func(t *testing.T) {
			outFile := path.Join(tmpDir, fmt.Sprintf("out%d.mp4", nr))
			args := []string{"mp4ff-decrypt"}
			if c.initFile != "" {
				args = append(args, "-init", c.initFile)
			}
			args = append(args, "-key", c.keyHexOrBase64, c.inFile, outFile)
			err := run(args)
			if err != nil {
				t.Error(err)
			}

			expectedOut, err := os.ReadFile(c.expectedOutFile)
			if err != nil {
				t.Error(err)
			}
			out, err := os.ReadFile(outFile)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(expectedOut, out) {
				t.Error("output file does not match expected")
			}
		})
	}
}

func TestParseKeys(t *testing.T) {
	legacyKey := "00112233445566778899aabbccddeeff"
	kidWithDash := "855ca997-b201-5736-f3d6-a59c9eff84d9"
	kidNoDash := "855ca997b2015736f3d6a59c9eff84d9"

	t.Run("legacy key", func(t *testing.T) {
		key, keysByKID, strictMode, err := parseKeys([]string{legacyKey})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strictMode {
			t.Fatal("expected non-strict mode")
		}
		if len(key) != 16 {
			t.Fatalf("unexpected key length: %d", len(key))
		}
		if len(keysByKID) != 0 {
			t.Fatalf("expected no kid map, got %d", len(keysByKID))
		}
	})

	t.Run("duplicate kid fails", func(t *testing.T) {
		_, _, _, err := parseKeys([]string{kidWithDash + ":" + legacyKey, kidNoDash + ":" + legacyKey})
		if err == nil {
			t.Fatal("expected duplicate kid error")
		}
		if !strings.Contains(err.Error(), "duplicate kid") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("mixed mode fails", func(t *testing.T) {
		_, _, _, err := parseKeys([]string{legacyKey, kidNoDash + ":" + legacyKey})
		if err == nil {
			t.Fatal("expected strict mixed mode error")
		}
		if !strings.Contains(err.Error(), "cannot mix") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestStrictKIDKeySelection(t *testing.T) {
	inFile := "../../mp4/testdata/cbcs_audio.mp4"
	expectedOutFile := "../../mp4/testdata/cbcs_audiodec.mp4"
	rawKey := "5ffd93861fa776e96cccd934898fc1c8"
	tmpDir := t.TempDir()
	outFile := path.Join(tmpDir, "outfile.mp4")

	input, err := mp4.ReadMP4File(inFile)
	if err != nil {
		t.Fatal(err)
	}
	decInfo, err := mp4.DecryptInit(input.Init)
	if err != nil {
		t.Fatal(err)
	}
	if len(decInfo.TrackInfos) == 0 || decInfo.TrackInfos[0].Sinf == nil {
		t.Fatal("missing encrypted track info")
	}
	kid := decInfo.TrackInfos[0].Sinf.Schi.Tenc.DefaultKID
	kidHex := hex.EncodeToString(kid)

	t.Run("matching kid decrypts", func(t *testing.T) {
		args := []string{"mp4ff-decrypt", "-key", kidHex + ":" + rawKey, inFile, outFile}
		err := run(args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedOut, err := os.ReadFile(expectedOutFile)
		if err != nil {
			t.Fatal(err)
		}
		out, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(expectedOut, out) {
			t.Fatal("output file does not match expected")
		}
	})

	t.Run("missing kid fails", func(t *testing.T) {
		missingKID := kidHex
		if missingKID[0] == '0' {
			missingKID = "1" + missingKID[1:]
		} else {
			missingKID = "0" + missingKID[1:]
		}
		args := []string{"mp4ff-decrypt", "-key", missingKID + ":" + rawKey, inFile, outFile}
		err := run(args)
		if err == nil {
			t.Fatal("expected missing kid error")
		}
		if !strings.Contains(err.Error(), "requested key was not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func BenchmarkDecodeCenc(b *testing.B) {
	inFile := "../../mp4/testdata/prog_8s_enc_dashinit.mp4"
	hexKey := "63cb5f7184dd4b689a5c5ff11ee6a328"
	raw, err := os.ReadFile(inFile)
	if err != nil {
		b.Error(err)
	}
	outData := make([]byte, 0, len(raw))
	outBuf := bytes.NewBuffer(outData)
	for i := 0; i < b.N; i++ {
		inBuf := bytes.NewBuffer(raw)
		outBuf.Reset()
		key, err := mp4.UnpackKey(hexKey)
		if err != nil {
			b.Error(err)
		}
		err = decryptFile(inBuf, nil, outBuf, key)
		if err != nil {
			b.Error(err)
		}
	}
}

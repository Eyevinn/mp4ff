package mp4

import "testing"

// For information about encryption, see https://github.com/gpac/gpac/wiki/Common-Encryption

func TestEncryptDecrypt(t *testing.T) {

	sample_txt := "0123456789abcdef0123456789abcdef!#%"
	iv := []byte("0123456776543210")
	key := []byte("00112233445566778899aabbccddeeff")

	sample_enc, err := DecryptSampleCTR([]byte(sample_txt), key, iv)
	if err != nil {
		t.Error(err)
	}
	sample_dec, err := DecryptSampleCTR(sample_enc, key, iv)
	if err != nil {
		t.Error(err)
	}
	sample_txt_dec := string(sample_dec)
	if sample_txt_dec != sample_txt {
		t.Errorf("Got %q instead of %q", sample_txt_dec, sample_txt)
	}

}

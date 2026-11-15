package media

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

// Signed file URLs (W5-3) let P-lane workers fetch attached images without an
// operator token: each payload URL carries an expiry and an HMAC over
// "media-file:{id}:{exp}". Keys are the vault key bytes, plumbed from main.
func SignFileURL(key []byte, id int64, expUnix int64) string {
	msg := fmt.Sprintf("media-file:%d:%d", id, expUnix)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	return fmt.Sprintf("/api/media/%d/signed?exp=%d&sig=%s",
		id, expUnix, hex.EncodeToString(mac.Sum(nil)))
}

// VerifyFileURL checks a signed URL. Expired or malformed URLs fail closed.
func VerifyFileURL(key []byte, id int64, expRaw, sig string) bool {
	if len(key) == 0 || sig == "" {
		return false
	}
	exp, err := strconv.ParseInt(expRaw, 10, 64)
	if err != nil || time.Unix(exp, 0).Before(time.Now()) {
		return false
	}
	msg := fmt.Sprintf("media-file:%d:%d", id, exp)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	want := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(sig), []byte(want)) == 1
}

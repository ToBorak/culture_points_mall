package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func CodeFor(activityID int64, window int64, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d|%d", activityID, window)
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

func ValidCode(activityID int64, code string, windowSecs int, secret string, now time.Time) bool {
	cur := now.Unix() / int64(windowSecs)
	if hmac.Equal([]byte(CodeFor(activityID, cur, secret)), []byte(code)) {
		return true
	}
	if hmac.Equal([]byte(CodeFor(activityID, cur-1, secret)), []byte(code)) {
		return true
	}
	return false
}

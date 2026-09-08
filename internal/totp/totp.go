package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
)

const (
	Digits = 6
	Period = 30
	Skew   = 1
)

var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func NewSecret() ([]byte, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func Encode(secret []byte) string {
	return encoding.EncodeToString(secret)
}

func Decode(s string) ([]byte, error) {
	cleaned := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	cleaned = strings.TrimRight(cleaned, "=")
	return encoding.DecodeString(cleaned)
}

func Generate(secret []byte, step uint64) string {
	counter := make([]byte, 8)
	binary.BigEndian.PutUint64(counter, step)

	mac := hmac.New(sha1.New, secret)
	mac.Write(counter)
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	binCode := uint32(sum[offset]&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])

	return fmt.Sprintf("%0*d", Digits, binCode%1_000_000)
}

func Step(now int64) uint64 {
	return uint64(now / Period)
}

func Validate(secret []byte, code string, now int64, lastStep uint64) (uint64, bool) {
	current := Step(now)
	for s := current - Skew; s <= current+Skew; s++ {
		if s <= lastStep {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(Generate(secret, s)), []byte(code)) == 1 {
			return s, true
		}
	}
	return 0, false
}

func URI(issuer, account, secret string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	query := strings.NewReplacer("+", "%20").Replace(url.Values{
		"secret":    {secret},
		"issuer":    {issuer},
		"algorithm": {"SHA1"},
		"digits":    {fmt.Sprint(Digits)},
		"period":    {fmt.Sprint(Period)},
	}.Encode())
	return "otpauth://totp/" + label + "?" + query
}

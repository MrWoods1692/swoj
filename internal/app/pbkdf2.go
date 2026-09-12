package app

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
)

// 认证错误。
var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUsernameExists     = errors.New("该用户名已被注册")
)

// pbkdf2SHA256 标准 PBKDF2-SHA256 实现（Go 1.22 标准库未内置 pbkdf2 包）。
func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	prf := func(key, data []byte) []byte {
		h := hmac.New(sha256.New, key)
		h.Write(data)
		return h.Sum(nil)
	}
	hashLen := sha256.Size
	numBlocks := (keyLen + hashLen - 1) / hashLen
	var out []byte
	var be [4]byte
	for block := 1; block <= numBlocks; block++ {
		binary.BigEndian.PutUint32(be[:], uint32(block))
		u := prf(password, append(append([]byte{}, salt...), be[:]...))
		t := make([]byte, len(u))
		copy(t, u)
		for i := 1; i < iter; i++ {
			u = prf(password, u)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

// hashPassword 使用 PBKDF2-SHA256 加盐存储口令，避免明文落地。
func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	dk := pbkdf2SHA256([]byte(password), salt, 120000, 32)
	return "pbkdf2$120000$" + base64.StdEncoding.EncodeToString(salt) + "$" +
		base64.StdEncoding.EncodeToString(dk), nil
}

// verifyPassword 恒定时间比较，防止时序侧信道。
func verifyPassword(password, stored string) bool {
	parts := strings.SplitN(stored, "$", 4)
	if len(parts) != 4 || parts[0] != "pbkdf2" {
		return false
	}
	iterations, err := parseInt(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	dk := pbkdf2SHA256([]byte(password), salt, iterations, len(want))
	return subtle.ConstantTimeCompare(dk, want) == 1
}

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("not int")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// hexID 生成短随机十六进制标识，用于 CSRF 令牌等一次性凭据。
func hexID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

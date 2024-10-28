package internal

import (
	"crypto/md5"
	"encoding/base64"
	"io"
)

func GetBase64MD5FromStr(s string) string {
	hash := md5.New()
	hash.Write([]byte(s))
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

func GetBase64MD5FromReader(r io.Reader) (string, error) {
	hash := md5.New()
	_, err := io.Copy(hash, r)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(hash.Sum(nil)), nil
}

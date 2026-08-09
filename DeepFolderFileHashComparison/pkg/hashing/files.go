package hashing

import (
	"crypto/sha256"
	"io"
	"os"
)

type Hasher interface {
	HashFile(filePath string) ([]byte, error)
	HashSizeInBytes() uint32
}

type sha256Hasher struct{}

func NewSha256Hasher() *sha256Hasher {
	return &sha256Hasher{}
}

func (*sha256Hasher) HashFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

func (*sha256Hasher) HashSizeInBytes() uint32 {
	return 256
}

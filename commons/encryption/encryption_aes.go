package encryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"io"

	"github.com/cockroachdb/errors"
)

const (
	AesSaltLen       int = 16
	aesCTRBufferSize     = 1024 * 1024 // 1 MiB
)

func PadPkcs7(data []byte, blocksize int) []byte {
	if (len(data) % blocksize) == 0 {
		return data
	}

	n := blocksize - (len(data) % blocksize)
	pb := make([]byte, len(data)+n)
	copy(pb, data)
	copy(pb[len(data):], bytes.Repeat([]byte{byte(n)}, n))
	return pb
}

func EncryptAESCTR(data []byte, salt []byte, key []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	writerBuffer := &bytes.Buffer{}

	err := EncryptAESCTRReaderWriter(reader, writerBuffer, salt, key)
	if err != nil {
		return nil, err
	}

	return writerBuffer.Bytes(), nil
}

func DecryptAESCTR(data []byte, salt []byte, key []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	writerBuffer := &bytes.Buffer{}

	err := DecryptAESCTRReaderWriter(reader, writerBuffer, salt, key)
	if err != nil {
		return nil, err
	}

	return writerBuffer.Bytes(), nil
}

func EncryptAESCTRReaderWriter(reader io.Reader, writer io.Writer, salt []byte, key []byte) error {
	paddedKey := PadPkcs7(key, 32)
	block, err := aes.NewCipher([]byte(paddedKey))
	if err != nil {
		return errors.Wrapf(err, "failed to create AES cipher")
	}

	streamCipher := cipher.NewCTR(block, salt)
	streamWriter := &cipher.StreamWriter{S: streamCipher, W: writer}

	_, err = io.CopyBuffer(streamWriter, reader, make([]byte, aesCTRBufferSize))
	return err
}

func DecryptAESCTRReaderWriter(reader io.Reader, writer io.Writer, salt []byte, key []byte) error {
	paddedKey := PadPkcs7(key, 32)
	block, err := aes.NewCipher([]byte(paddedKey))
	if err != nil {
		return errors.Wrapf(err, "failed to create AES cipher")
	}

	streamCipher := cipher.NewCTR(block, salt)
	streamWriter := &cipher.StreamWriter{S: streamCipher, W: writer}

	_, err = io.CopyBuffer(streamWriter, reader, make([]byte, aesCTRBufferSize))
	return err
}

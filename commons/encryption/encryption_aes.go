package encryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"io"

	"github.com/cockroachdb/errors"
)

const (
	AesSaltLen int = 16
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

	decrypter := cipher.NewCTR(block, salt)

	buf := make([]byte, block.BlockSize())
	destBuf := make([]byte, block.BlockSize())
	for {
		readLen, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		decrypter.XORKeyStream(destBuf, buf[:readLen])
		writeLen, err := writer.Write(destBuf[:readLen])
		if err != nil {
			return err
		}

		if writeLen != readLen {
			return errors.Errorf("failed to write")
		}
	}
}

func DecryptAESCTRReaderWriter(reader io.Reader, writer io.Writer, salt []byte, key []byte) error {
	paddedKey := PadPkcs7(key, 32)
	block, err := aes.NewCipher([]byte(paddedKey))
	if err != nil {
		return errors.Wrapf(err, "failed to create AES cipher")
	}

	decrypter := cipher.NewCTR(block, salt)

	buf := make([]byte, block.BlockSize())
	destBuf := make([]byte, block.BlockSize())
	for {
		readLen, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		decrypter.XORKeyStream(destBuf, buf[:readLen])
		writeLen, err := writer.Write(destBuf[:readLen])
		if err != nil {
			return err
		}

		if writeLen != readLen {
			return errors.Errorf("failed to write")
		}
	}
}

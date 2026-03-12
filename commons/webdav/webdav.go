package webdav

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/avast/retry-go"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_common "github.com/cyverse/go-irodsclient/irods/common"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/cyverse/gocommands/commons/types"
	log "github.com/sirupsen/logrus"
	"github.com/studio-b12/gowebdav"
)

type WebDAVClient struct {
	baseURL  string
	username string
	password string
}

func NewWebDAVClient(baseURL string, username string, password string) *WebDAVClient {
	return &WebDAVClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
	}
}

func (client *WebDAVClient) getWebDAVErrorCode(err error) (int, bool) {
	if pe, ok := err.(*os.PathError); ok {
		if se, ok2 := pe.Err.(gowebdav.StatusError); ok2 {
			return se.Status, true
		}
	}
	return 0, false
}

func (client *WebDAVClient) getWebDavError(url string, err error) error {
	if err == nil {
		return nil
	}
	if errorCode, ok := client.getWebDAVErrorCode(err); ok {
		return types.NewWebDAVError(url, errorCode)
	}
	return err
}

func (client *WebDAVClient) DownloadFile(sourceEntry *irodsclient_fs.Entry, localPath string, callback irodsclient_common.TransferTrackerCallback) (*irodsclient_fs.FileTransferResult, error) {
	logger := log.WithFields(log.Fields{
		"irods_source_path": sourceEntry.Path,
		"local_path":        localPath,
	})

	irodsSrcPath := irodsclient_util.GetCorrectIRODSPath(sourceEntry.Path)
	localDestPath := irodsclient_util.GetCorrectLocalPath(localPath)

	localFilePath := localDestPath

	fileTransferResult := &irodsclient_fs.FileTransferResult{}
	fileTransferResult.IRODSPath = irodsSrcPath
	fileTransferResult.StartTime = time.Now()

	stat, err := os.Stat(localDestPath)
	if err != nil {
		if os.IsNotExist(err) {
			// file not exists, it's a file
			// pass
		} else {
			return fileTransferResult, err
		}
	} else {
		if stat.IsDir() {
			irodsFileName := irodsclient_util.GetIRODSPathFileName(irodsSrcPath)
			localFilePath = filepath.Join(localDestPath, irodsFileName)
		}
	}

	fileTransferResult.LocalPath = localFilePath
	fileTransferResult.IRODSCheckSumAlgorithm = sourceEntry.CheckSumAlgorithm
	fileTransferResult.IRODSCheckSum = sourceEntry.CheckSum
	fileTransferResult.IRODSSize = sourceEntry.Size

	if len(sourceEntry.CheckSum) == 0 {
		return fileTransferResult, errors.Errorf("failed to get checksum of the source file for path %q", irodsSrcPath)
	}

	if sourceEntry.Size == 0 {
		// zero size file, just create an empty file
		f, err := os.OpenFile(localPath, os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			return fileTransferResult, errors.Wrapf(err, "failed to create a local file %q", localPath)
		}
		f.Close()

		fileTransferResult.LocalCheckSumAlgorithm = sourceEntry.CheckSumAlgorithm
		fileTransferResult.LocalCheckSum = sourceEntry.CheckSum
		fileTransferResult.LocalSize = 0

		fileTransferResult.EndTime = time.Now()

		return fileTransferResult, nil
	}

	webdav := gowebdav.NewClient(client.baseURL, client.username, client.password)

	tlsConfig := &tls.Config{
		CipherSuites: []uint16{
			// TLS 1.0 - 1.2 cipher suites.
			tls.TLS_RSA_WITH_RC4_128_SHA,
			tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
			tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			// TLS 1.3 cipher suites.
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	webdav.SetTransport(transport)
	err = webdav.Connect()
	if err != nil {
		if httpStatusErr, ok := client.getWebDAVErrorCode(err); ok {
			return fileTransferResult, types.NewWebDAVError(client.baseURL, int(httpStatusErr))
		}

		return fileTransferResult, types.NewWebDAVError(client.baseURL, http.StatusServiceUnavailable)
	}

	// download the file
	offset := int64(0)
	readSize := sourceEntry.Size
	for offset < sourceEntry.Size {
		download := func() error {
			readSize = sourceEntry.Size - offset

			logger.Debugf("downloading file %s (offset %d, length %d) from WebDAV server", irodsSrcPath, offset, readSize)

			reader, readErr := webdav.ReadStreamRange(irodsSrcPath, offset, readSize)
			if readErr != nil {
				baseErr := client.getWebDavError(client.baseURL+irodsSrcPath, readErr)
				return errors.Wrapf(baseErr, "failed to read stream range of file %q (offset %d, length %d) from WebDAV server", irodsSrcPath, offset, readSize)
			}
			defer reader.Close()

			newOffset, downloadErr := client.downloadToLocalWithTrackerCallBack(reader, localFilePath, offset, readSize, sourceEntry.Size, callback)
			if downloadErr != nil {
				logger.WithError(downloadErr).Debugf("failed to download file %q (offset %d, length %d) from WebDAV server", irodsSrcPath, offset, readSize)

				// if the download failed, we need to update the offset
				offset = newOffset
				return errors.Wrapf(downloadErr, "failed to download file %q (offset %d, length %d) from WebDAV server", irodsSrcPath, offset, readSize)
			}

			offset = newOffset
			return nil
		}

		// retry download in case of failure
		// we retry 3 times with 5 seconds delay between attempts
		offsetLast := offset
		retryErr := retry.Do(download, retry.Attempts(3), retry.Delay(5*time.Second), retry.LastErrorOnly(true))
		if retryErr != nil {
			if errors.Is(retryErr, io.ErrUnexpectedEOF) && offsetLast < offset {
				// progress was made
				// continue to retry
				logger.WithError(retryErr).Debugf("downloaded file %q (offset %d, length %d, data left %d) from WebDAV server, but got unexpected EOF, retrying...", irodsSrcPath, offset, offset-offsetLast, readSize)
			} else {
				return fileTransferResult, errors.Wrapf(retryErr, "failed to download file %q (offset %d, length %d) from WebDAV server after 3 attempts", irodsSrcPath, offset, readSize)
			}
		}
	}

	fileTransferResult.LocalSize = offset

	localHash, err := client.calculateLocalFileHash(localPath, sourceEntry.CheckSumAlgorithm, callback)
	if err != nil {
		return fileTransferResult, errors.Wrapf(err, "failed to calculate hash of local file %q with alg %s", localPath, sourceEntry.CheckSumAlgorithm)
	}

	fileTransferResult.LocalCheckSumAlgorithm = sourceEntry.CheckSumAlgorithm
	fileTransferResult.LocalCheckSum = localHash

	if !bytes.Equal(sourceEntry.CheckSum, localHash) {
		return fileTransferResult, errors.Errorf("checksum verification failed for local file %q, download failed", localPath)
	}

	fileTransferResult.EndTime = time.Now()

	return fileTransferResult, nil
}

func (client *WebDAVClient) calculateLocalFileHash(localPath string, algorithm irodsclient_types.ChecksumAlgorithm, processCallback irodsclient_common.TransferTrackerCallback) ([]byte, error) {
	// verify checksum
	hashBytes, err := irodsclient_util.HashLocalFile(localPath, string(algorithm), processCallback)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get %q hash of %q", algorithm, localPath)
	}

	return hashBytes, nil
}

func (client *WebDAVClient) downloadToLocalWithTrackerCallBack(reader io.ReadCloser, localPath string, offset int64, readLength int64, fileSize int64, callback irodsclient_common.TransferTrackerCallback) (int64, error) {
	f, err := os.OpenFile(localPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return offset, errors.Wrapf(err, "failed to open local file %q", localPath)
	}
	defer f.Close()

	newOffset, err := f.Seek(offset, io.SeekStart)
	if err != nil {
		return offset, errors.Wrapf(err, "failed to seek to offset %d in local file %q", offset, localPath)
	}

	if newOffset != offset {
		return offset, errors.Errorf("failed to seek to offset %d in local file %q, current offset is %d", offset, localPath, newOffset)
	}

	if callback != nil {
		callback("download", offset, fileSize)
	}

	sizeLeft := readLength
	actualRead := int64(0)
	actualWrite := int64(0)

	buffer := make([]byte, 64*1024) // 64KB buffer
	for sizeLeft > 0 {
		sizeRead, err := reader.Read(buffer)

		if sizeRead > 0 {
			sizeLeft -= int64(sizeRead)
			actualRead += int64(sizeRead)

			sizeWritten, writeErr := f.Write(buffer[:sizeRead])
			if writeErr != nil {
				return offset + actualWrite, errors.Wrapf(writeErr, "failed to write to local file %q", localPath)
			}

			if sizeWritten != sizeRead {
				return offset + actualWrite, errors.Errorf("failed to write all bytes to local file %q, expected %d, got %d", localPath, sizeRead, sizeWritten)
			}

			actualWrite += int64(sizeWritten)

			if callback != nil {
				callback("download", offset+actualWrite, fileSize)
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}

			return offset + actualWrite, errors.Wrapf(err, "failed to read from reader")
		}
	}

	if actualWrite != readLength {
		return offset + actualWrite, errors.Errorf("file size mismatch, expected %d, got %d", readLength, actualRead)
	}

	return offset + actualWrite, nil
}

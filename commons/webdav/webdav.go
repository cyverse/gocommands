package webdav

import (
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_common "github.com/cyverse/go-irodsclient/irods/common"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/go-irodsclient/irods/util"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/cyverse/gocommands/commons/types"
	log "github.com/sirupsen/logrus"
	"github.com/studio-b12/gowebdav"
)

type WebDAVClient struct {
	filesystem *irodsclient_fs.FileSystem
	baseURL    string
	username   string
	password   string

	webdav *gowebdav.Client
}

func NewWebDAVClient(filesystem *irodsclient_fs.FileSystem, baseURL string, username string, password string) (*WebDAVClient, error) {
	client := &WebDAVClient{
		filesystem: filesystem,
		baseURL:    strings.TrimRight(baseURL, "/"),
		username:   username,
		password:   password,

		webdav: nil,
	}

	err := client.initWebDAV()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to WebDAV server")
	}
	return client, nil
}

func (client *WebDAVClient) initWebDAV() error {
	webdav := gowebdav.NewClient(client.baseURL, client.username, client.password)

	webdav.SetTransport(newWebDAVTransport())
	err := webdav.Connect()
	if err != nil {
		if httpStatusErr, ok := client.getWebDAVErrorCode(err); ok {
			return types.NewWebDAVError(client.baseURL, int(httpStatusErr))
		}

		return types.NewWebDAVError(client.baseURL, http.StatusServiceUnavailable)
	}

	client.webdav = webdav
	return nil
}

func newWebDAVTransport() *http.Transport {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS10,
		CipherSuites: []uint16{
			// TLS 1.0 - 1.2 cipher suites, retained for iRODS WebDAV compatibility.
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
			// TLS 1.3 cipher suites are not configurable in Go, but are kept for
			// compatibility with the previous configuration.
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}

	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSClientConfig:       tlsConfig,
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

func (client *WebDAVClient) getPathForTicket(irodsPath string, ticket string) string {
	webdavURL, err := url.Parse(client.baseURL)
	if err != nil {
		return client.baseURL + irodsPath + "?ticket=" + url.QueryEscape(ticket)
	}
	webdavURL.Path = strings.TrimRight(webdavURL.Path, "/") + "/" + strings.TrimLeft(irodsPath, "/")
	query := webdavURL.Query()
	query.Set("ticket", ticket)
	webdavURL.RawQuery = query.Encode()
	return webdavURL.String()
}

func (client *WebDAVClient) DownloadFile(sourceEntry *irodsclient_fs.Entry, localPath string, ticket string, verifyChecksum bool, callback irodsclient_common.TransferTrackerCallback) (*irodsclient_fs.FileTransferResult, error) {
	logger := log.WithFields(log.Fields{
		"irods_source_path": sourceEntry.Path,
		"local_path":        localPath,
		"ticket":            ticket,
	})

	irodsSrcPath := irodsclient_util.CleanIRODSPath(sourceEntry.Path)
	localDestPath := irodsclient_util.CleanLocalPath(localPath)

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

	if verifyChecksum {
		if len(sourceEntry.CheckSum) == 0 {
			return fileTransferResult, errors.Errorf("failed to get checksum of the source file for path %q", irodsSrcPath)
		}
	}

	if sourceEntry.Size == 0 {
		// zero size file, just create an empty file
		f, err := os.OpenFile(localFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			return fileTransferResult, errors.Wrapf(err, "failed to create a local file %q", localFilePath)
		}
		if err := f.Close(); err != nil {
			return fileTransferResult, errors.Wrapf(err, "failed to close a local file %q", localFilePath)
		}

		fileTransferResult.LocalCheckSumAlgorithm = sourceEntry.CheckSumAlgorithm
		fileTransferResult.LocalCheckSum = sourceEntry.CheckSum
		fileTransferResult.LocalSize = 0

		fileTransferResult.EndTime = time.Now()

		return fileTransferResult, nil
	}

	offset := int64(0)
	resumed := false
	if partialStat, statErr := os.Stat(localFilePath); statErr == nil {
		if partialStat.Size() > 0 && partialStat.Size() < sourceEntry.Size {
			if len(sourceEntry.CheckSum) > 0 {
				offset = partialStat.Size()
				resumed = true
				logger.Debugf("resuming download of %q from offset %d", irodsSrcPath, offset)
			} else {
				logger.Warnf("checksum for %q is not available; restarting instead of using an unverifiable local partial file", irodsSrcPath)
			}
		}
	}

	readSize := sourceEntry.Size - offset

	logger.Debugf("downloading file %s (offset %d, length %d) from WebDAV server", irodsSrcPath, offset, readSize)

	newOffset, downloadErr := client.downloadToLocalWithTrackerCallBack(irodsSrcPath, localFilePath, ticket, offset, readSize, sourceEntry.Size, callback)
	if downloadErr != nil {
		logger.WithError(downloadErr).Debugf("failed to download file %q (offset %d, length %d) from WebDAV server", irodsSrcPath, offset, readSize)
		return fileTransferResult, errors.Wrapf(downloadErr, "failed to download file %q (offset %d, length %d) from WebDAV server", irodsSrcPath, offset, readSize)
	}

	offset = newOffset

	fileTransferResult.LocalSize = offset

	if verifyChecksum || resumed {
		localHash, err := client.calculateLocalFileHash(localFilePath, sourceEntry.CheckSumAlgorithm, callback)
		if err != nil {
			return fileTransferResult, errors.Wrapf(err, "failed to calculate hash of local file %q with alg %s", localFilePath, sourceEntry.CheckSumAlgorithm)
		}

		fileTransferResult.LocalCheckSumAlgorithm = sourceEntry.CheckSumAlgorithm
		fileTransferResult.LocalCheckSum = localHash

		if !bytes.Equal(sourceEntry.CheckSum, localHash) {
			if resumed {
				logger.Warnf("checksum verification failed after resuming %q; downloading the file again from the beginning", irodsSrcPath)
				newOffset, downloadErr = client.downloadToLocalWithTrackerCallBack(irodsSrcPath, localFilePath, ticket, 0, sourceEntry.Size, sourceEntry.Size, callback)
				if downloadErr != nil {
					return fileTransferResult, errors.Wrapf(downloadErr, "failed to re-download file %q from the beginning after checksum verification failed", irodsSrcPath)
				}
				fileTransferResult.LocalSize = newOffset

				localHash, err = client.calculateLocalFileHash(localFilePath, sourceEntry.CheckSumAlgorithm, callback)
				if err != nil {
					return fileTransferResult, errors.Wrapf(err, "failed to calculate hash of local file %q with alg %s", localFilePath, sourceEntry.CheckSumAlgorithm)
				}
				fileTransferResult.LocalCheckSum = localHash
				if bytes.Equal(sourceEntry.CheckSum, localHash) {
					fileTransferResult.EndTime = time.Now()
					return fileTransferResult, nil
				}
			}

			os.Remove(localFilePath)
			return fileTransferResult, errors.Errorf("checksum verification failed for local file %q, download failed", localFilePath)
		}
	}

	fileTransferResult.EndTime = time.Now()

	return fileTransferResult, nil
}

func (client *WebDAVClient) UploadFile(localPath string, irodsPath string, ticket string, verifyChecksum bool, callback irodsclient_common.TransferTrackerCallback) (*irodsclient_fs.FileTransferResult, error) {
	logger := log.WithFields(log.Fields{
		"local_source_path": localPath,
		"irods_path":        irodsPath,
		"ticket":            ticket,
	})

	localSrcPath := irodsclient_util.CleanLocalPath(localPath)
	irodsDestPath := irodsclient_util.CleanIRODSPath(irodsPath)

	irodsFilePath := irodsDestPath

	fileTransferResult := &irodsclient_fs.FileTransferResult{}
	fileTransferResult.LocalPath = localSrcPath
	fileTransferResult.StartTime = time.Now()

	stat, err := os.Stat(localSrcPath)
	if err != nil {
		if os.IsNotExist(err) {
			// file not exists
			newErr := errors.Join(err, irodsclient_types.NewFileNotFoundError(localSrcPath))
			return fileTransferResult, errors.Wrapf(newErr, "failed to find a file for local path %q", localSrcPath)
		}
		return fileTransferResult, err
	}

	if stat.IsDir() {
		newErr := irodsclient_types.NewFileNotFoundError(localSrcPath)
		return fileTransferResult, errors.Wrapf(newErr, "failed to find a file for local path %q, the path is for a directory", localSrcPath)
	}

	overwrite := false
	entry, err := client.filesystem.Stat(irodsDestPath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			return fileTransferResult, err
		}
	} else {
		if entry.IsDir() {
			localFileName := filepath.Base(localSrcPath)
			irodsFilePath = util.MakeIRODSPath(irodsDestPath, localFileName)
		} else {
			// overwrite the existing file
			// we do not need to delete the existing file before upload, as WebDAV WriteStream will overwrite the file if it already exists
			overwrite = true
		}
	}

	fileTransferResult.LocalSize = stat.Size()
	fileTransferResult.IRODSPath = irodsFilePath

	// upload the file
	writeSize := stat.Size()

	logger.Debugf("uploading file %s (length %d) to WebDAV server", localSrcPath, writeSize)

	_, uploadErr := client.uploadToIrodsWithTrackerCallBack(localSrcPath, irodsFilePath, ticket, writeSize, callback)
	if uploadErr != nil {
		logger.WithError(uploadErr).Debugf("failed to upload file %q (length %d) to WebDAV server", localSrcPath, writeSize)
		return fileTransferResult, errors.Wrapf(uploadErr, "failed to upload file %q (length %d) to WebDAV server", localSrcPath, writeSize)
	}

	if overwrite {
		// update - overwrite
		client.filesystem.InvalidateCacheForFileUpdate(irodsFilePath)
	} else {
		// create
		client.filesystem.InvalidateCacheForFileCreate(irodsFilePath)
	}

	entry, err = client.filesystem.Stat(irodsFilePath)
	if err != nil {
		return fileTransferResult, err
	}

	fileTransferResult.IRODSCheckSumAlgorithm = entry.CheckSumAlgorithm
	fileTransferResult.IRODSCheckSum = entry.CheckSum
	fileTransferResult.IRODSSize = entry.Size

	if verifyChecksum {
		if len(entry.CheckSum) == 0 {
			logger.Warnf("checksum for uploaded file %q is not available yet; skipping checksum verification", irodsFilePath)
			terminal.PrintErrorf("Warning: checksum for uploaded file %q is not available yet; skipping checksum verification\n", irodsFilePath)
		} else {
			localHash, err := client.calculateLocalFileHash(localSrcPath, entry.CheckSumAlgorithm, callback)
			if err != nil {
				return fileTransferResult, errors.Wrapf(err, "failed to calculate hash of local file %q with alg %s", localSrcPath, entry.CheckSumAlgorithm)
			}

			fileTransferResult.LocalCheckSumAlgorithm = entry.CheckSumAlgorithm
			fileTransferResult.LocalCheckSum = localHash

			err = validateUploadedChecksum(client.filesystem, irodsFilePath, entry.CheckSum, localHash)
			if err != nil {
				return fileTransferResult, err
			}
		}
	}

	fileTransferResult.EndTime = time.Now()

	return fileTransferResult, nil
}

type remoteFileRemover interface {
	RemoveFile(irodsPath string, force bool) error
}

func validateUploadedChecksum(filesystem remoteFileRemover, irodsFilePath string, expected []byte, actual []byte) error {
	if len(expected) == 0 {
		return nil
	}
	if bytes.Equal(expected, actual) {
		return nil
	}

	if err := filesystem.RemoveFile(irodsFilePath, true); err != nil {
		return errors.Wrapf(err, "checksum verification failed for iRODS file %q and failed to remove the corrupted file", irodsFilePath)
	}
	return errors.Errorf("checksum verification failed for iRODS file %q, removed corrupted file", irodsFilePath)
}

func (client *WebDAVClient) calculateLocalFileHash(localPath string, algorithm irodsclient_types.ChecksumAlgorithm, processCallback irodsclient_common.TransferTrackerCallback) ([]byte, error) {
	// verify checksum
	hashBytes, err := irodsclient_util.HashLocalFile(localPath, string(algorithm), processCallback)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get %q hash of %q", algorithm, localPath)
	}

	return hashBytes, nil
}

func (client *WebDAVClient) downloadToLocalWithTrackerCallBack(irodsPath string, localPath string, ticket string, offset int64, readLength int64, fileSize int64, callback irodsclient_common.TransferTrackerCallback) (int64, error) {
	webdavPath := client.getPathForTicket(irodsPath, ticket)

	reader, readErr := client.webdav.ReadStreamRange(webdavPath, offset, readLength)
	if readErr != nil {
		baseErr := client.getWebDavError(client.baseURL+irodsPath, readErr)
		return offset, errors.Wrapf(baseErr, "failed to read stream range of file %q (offset %d, length %d) from WebDAV server", irodsPath, offset, readLength)
	}
	defer reader.Close()

	flags := os.O_RDWR | os.O_CREATE
	if offset == 0 {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(localPath, flags, 0666)
	if err != nil {
		return offset, errors.Wrapf(err, "failed to open local file %q", localPath)
	}

	newOffset, err := f.Seek(offset, io.SeekStart)
	if err != nil {
		return offset, errors.Wrapf(err, "failed to seek to offset %d in local file %q", offset, localPath)
	}

	if newOffset != offset {
		return offset, errors.Errorf("failed to seek to offset %d in local file %q, current offset is %d", offset, localPath, newOffset)
	}

	actualWrite := int64(0)

	progress := func(writeSize int) {
		actualWrite += int64(writeSize)

		if callback != nil {
			callback("download", offset+actualWrite, fileSize)
		}
	}

	progressWriter := NewWriterWithProgress(f, progress)
	defer progressWriter.Close()

	if callback != nil {
		callback("download", offset, fileSize)
	}

	copied, err := io.CopyN(progressWriter, reader, readLength)
	if err != nil && !errors.Is(err, io.EOF) {
		return offset + actualWrite, errors.Wrapf(err, "failed to copy data to local file %q", localPath)
	}

	if readLength != copied {
		return offset + actualWrite, errors.Errorf("file size mismatch, expected %d, got %d", readLength, copied)
	}

	if readLength != actualWrite {
		return offset + actualWrite, errors.Errorf("file size mismatch, expected %d, got %d", readLength, actualWrite)
	}

	return offset + actualWrite, nil
}

func (client *WebDAVClient) uploadToIrodsWithTrackerCallBack(localPath string, irodsPath string, ticket string, fileSize int64, callback irodsclient_common.TransferTrackerCallback) (int64, error) {
	webdavPath := client.getPathForTicket(irodsPath, ticket)

	reader, readErr := os.Open(localPath)
	if readErr != nil {
		return 0, errors.Wrapf(readErr, "failed to open local file %q", localPath)
	}

	if callback != nil {
		callback("upload", 0, fileSize)
	}

	actualRead := int64(0)

	progress := func(readSize int) {
		actualRead += int64(readSize)

		if callback != nil {
			callback("upload", actualRead, fileSize)
		}
	}

	progressReader := NewReaderWithProgress(reader, progress)
	defer progressReader.Close()

	err := client.webdav.WriteStreamWithLength(webdavPath, progressReader, fileSize, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return actualRead, errors.Wrapf(err, "failed to copy data to irods file %q", irodsPath)
	}

	if actualRead != fileSize {
		return actualRead, errors.Errorf("file size mismatch, expected %d, got %d", fileSize, actualRead)
	}

	return actualRead, nil
}

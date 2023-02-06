package commons

const (
	// add ppid to file name
	BundleLogDir      string = ".gocmd"
	BundleLogFileName string = "bundle_log.json"
)

type BundleFile struct {
	LocalPath string `json:"local_path"`
	IRODSPath string `json:"irods_path"`
	Size      int64  `json:"size"`
}

// BundleLogRecord stores an information of bundle created
type BundleLogRecord struct {
	BundleID string       `json:"id"`
	Files    []BundleFile `json:"files"`
}

package commons

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

var (
	clientVersion string
	gitCommit     string
	buildDate     string
)

// VersionInfo object contains version related info
type VersionInfo struct {
	ClientVersion string `json:"clientVersion"`
	GitCommit     string `json:"gitCommit"`
	BuildDate     string `json:"buildDate"`
	GoVersion     string `json:"goVersion"`
	Compiler      string `json:"compiler"`
	Platform      string `json:"platform"`
}

// GetVersion returns VersionInfo object
func GetVersion() VersionInfo {
	return VersionInfo{
		ClientVersion: clientVersion,
		GitCommit:     gitCommit,
		BuildDate:     buildDate,
		GoVersion:     runtime.Version(),
		Compiler:      runtime.Compiler,
		Platform:      fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// GetClientVersion returns client version in string
func GetClientVersion() string {
	return clientVersion
}

// GetVersionJSON returns VersionInfo object in JSON string
func GetVersionJSON() (string, error) {
	info := GetVersion()
	marshalled, err := json.MarshalIndent(&info, "", "  ")
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal version info to json")
	}
	return string(marshalled), nil
}

// GetVersionParts returns version parts (major, minor, patch)
func GetVersionParts(version string) (int, int, int) {
	major := 0
	minor := 0
	patch := 0

	if len(version) == 0 {
		return 0, 0, 0
	}

	version = version[1:]

	version = strings.ToLower(version)
	version = strings.TrimPrefix(version, "v")

	vers := strings.Split(version, ".")
	if len(vers) >= 1 {
		m, err := strconv.Atoi(vers[0])
		if err == nil {
			major = m
		}
	}

	if len(vers) >= 2 {
		m, err := strconv.Atoi(vers[1])
		if err == nil {
			minor = m
		}
	}

	if len(vers) >= 3 {
		p, err := strconv.Atoi(vers[2])
		if err == nil {
			patch = p
		}
	}

	return major, minor, patch
}

// IsNewerVersion compares ver1 against ver2
func IsNewerVersion(ver1 []int, ver2 []int) bool {
	if len(ver1) < 3 || len(ver2) < 3 {
		return false
	}

	if ver1[0] > ver2[0] {
		return true
	}
	if ver1[0] < ver2[0] {
		return false
	}
	// major is equal
	if ver1[1] > ver2[1] {
		return true
	}
	if ver1[1] < ver2[1] {
		return false
	}
	// minor is equal
	if ver1[2] > ver2[2] {
		return true
	}
	if ver1[2] < ver2[2] {
		return false
	}
	return false
}

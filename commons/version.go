package commons

import (
	"encoding/json"
	"fmt"
	"runtime"
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
		return "", err
	}
	return string(marshalled), nil
}

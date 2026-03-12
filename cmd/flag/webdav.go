package flag

import (
	"github.com/spf13/cobra"
)

type WebDAVFlagValues struct {
	WebDAV bool
}

var (
	webdavFlagValues WebDAVFlagValues
)

func SetWebDAVFlags(command *cobra.Command) {
	command.Flags().BoolVar(&webdavFlagValues.WebDAV, "webdav", false, "Use WebDAV protocol for operations")
}

func GetWebDAVFlagValues() *WebDAVFlagValues {
	return &webdavFlagValues
}

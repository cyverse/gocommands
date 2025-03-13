package commons

type TransferMode string

const (
	TransferModeSingleThread TransferMode = "single_thread"
	TransferModeICAT         TransferMode = "icat"
	TransferModeRedirect     TransferMode = "redirect"
)

func (t TransferMode) Valid() bool {
	if t == TransferModeSingleThread || t == TransferModeICAT || t == TransferModeRedirect {
		return true
	}
	return false
}

package commons

type TransferMode string

const (
	TransferModeICAT     TransferMode = "icat"
	TransferModeRedirect TransferMode = "redirect"
)

func (t TransferMode) Valid() bool {
	if t == TransferModeICAT || t == TransferModeRedirect {
		return true
	}
	return false
}

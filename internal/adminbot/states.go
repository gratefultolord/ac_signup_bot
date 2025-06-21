package adminbot

type AdminState struct {
	Step      string
	RequestID int64
}

const (
	StateMainMenu = "main_menu"

	StateViewingRequest = "viewing_request"

	StateEnteringRejectReason   = "entering_reject_reason"
	StateEnteringRevisionReason = "entering_revision_reason"

	StateAddingAdmin = "adding_admin"
)

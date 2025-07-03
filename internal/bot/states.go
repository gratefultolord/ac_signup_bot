package bot

import "time"

type UserState struct {
	Step                          string
	FirstName                     string
	LastName                      string
	BirthDate                     time.Time
	UserStatus                    string
	DocumentPath                  string
	PhoneNumber                   string
	RequestID                     int64
	MessageDraft                  string
	WaitingForPrivacyConfirmation bool
}

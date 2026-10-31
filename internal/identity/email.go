package identity

import (
	"fmt"
	"net/mail"

	"github.com/lidisoft/horizon/internal/identity/db"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
)

// Emails are English only for now (ADR-0017); they move to the server-side
// catalogs in internal/i18n with notifications (phase 8).

func resetMessage(u db.User, link string) platformmail.Message {
	return platformmail.Message{
		To:      &mail.Address{Name: u.Name, Address: u.Email},
		Subject: "Reset your Horizon password",
		Body: fmt.Sprintf(`Hello %s,

Someone asked to reset the password of your Horizon account. To choose a new
password, open this link within an hour:

%s

If it wasn't you, ignore this email: your password stays the same.
`, u.Name, link),
	}
}

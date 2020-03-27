package util

import (
	"github.com/mongodb/grip/send"
)

func GetSystemSender() send.Sender { return getSystemLogger() }

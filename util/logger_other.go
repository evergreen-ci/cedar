// +build !linux

package util

import "github.com/mongodb/grip/send"

func getSystemLogger() send.Sender { return send.MakeNative() }

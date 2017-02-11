package bsonutil

import "strings"

func GetDottedKeyName(fieldNames ...string) string {
	return strings.Join(fieldNames, ".")
}

package model

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
)

type PerformanceResultID struct {
	TaskName  string
	Execution int
	TestName  string
	Parent    string
	Tags      []string
}

func (id *PerformanceResultID) ID() string {
	buf := &bytes.Buffer{}
	buf.WriteString(id.TaskName)
	buf.WriteString(fmt.Sprint(id.Execution))
	buf.WriteString(id.TestName)
	buf.WriteString(id.Parent)
	sort.Strings(id.Tags)
	for _, str := range id.Tags {
		buf.WriteString(str)
	}

	hash := sha256.New()

	return string(hash.Sum(buf.Bytes()))
}

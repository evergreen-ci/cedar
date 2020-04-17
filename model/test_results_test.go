package model

import (
	"math/rand"
	"time"
)

func getTestResults() *TestResults {
	r := &TestResult{}
}

func getTestResult() *TestResult {
	return &TestResult{
		TestName:       seedRand.String(10),
		TestFile:       rand.String(10),
		Trial:          seedRand.Intn(10),
		Status:         "Pass",
		LineNum:        seedRand.Int(1000),
		TaskCreateTime: time.Now().Add(-time.Hour),
		TaskStartTime:  time.Now().Add(-30 * time.Hour),
		TaskEndTime:    time.Now(),
	}
}

var seedRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

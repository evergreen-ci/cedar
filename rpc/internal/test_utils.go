package internal

var globalPortCounter = 3999

func getPort() int {
	globalPortCounter += 1
	return globalPortCounter
}

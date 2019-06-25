package internal

var globalPortChan = make(chan int)

func init() {
	go func() {
		port := 4000
		for {
			globalPortChan <- port
			port++
		}
	}()
}

func getPort() int {
	return <-globalPortChan
}

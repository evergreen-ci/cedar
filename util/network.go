package util

import (
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

func GetPublicIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", errors.Wrap(err, "could not establish outbound connection")
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	localAddrStr := localAddr.IP.To4().String()
	if strings.HasPrefix(localAddrStr, "172.17") {

		resp, err := http.DefaultClient.Get("http://169.254.169.254/latest/meta-data/public-ipv4")
		if err != nil {
			return localAddrStr, errors.Wrap(err, "problem accessing metadata service")
		}
		defer resp.Body.Close()

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return localAddrStr, errors.Wrap(err, "problem reading response body")
		}

		localAddrStr = string(data)
	}

	return localAddrStr, nil
}

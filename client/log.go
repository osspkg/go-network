package client

import "go.osspkg.com/logx"

func writeLog(err error, message, network, address string) {
	if err == nil {
		return
	}
	logx.Error(message, "err", err, "network", network, "address", address)
}

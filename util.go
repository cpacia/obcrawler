package main

import (
	"gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
)

type NodeType int

const (
	Clearnet NodeType = iota
	TorOnly
	DualStack
)

func GetNodeType(addrs []multiaddr.Multiaddr) NodeType {
	usingTor := false
	usingClearnet := false
	for _, addr := range addrs {
		clear := true
		for _, protocol := range addr.Protocols() {
			if protocol.Code == multiaddr.P_ONION {
				usingTor = true
				clear = false
				break
			}
		}
		if clear {
			usingClearnet = true
		}
	}
	switch {
	case usingClearnet && !usingTor:
		return Clearnet
	case usingTor && !usingClearnet:
		return TorOnly
	case usingTor && usingClearnet:
		return DualStack
	default:
		return Clearnet
	}
}
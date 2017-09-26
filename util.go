package main

import (
	"gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	"errors"
)

type NodeType int

const (
	Clearnet NodeType = iota
	TorOnly
	DualStack
)

func GetNodeType(addrs []multiaddr.Multiaddr) (NodeType, error) {
	if len(addrs) == 0 {
		return Clearnet, errors.New("No addrs provided")
	}
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
		return Clearnet, nil
	case usingTor && !usingClearnet:
		return TorOnly, nil
	case usingTor && usingClearnet:
		return DualStack, nil
	default:
		return Clearnet, errors.New("Error parsing addrs")
	}
}
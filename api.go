package main

import (
	"encoding/json"
	"net/http"
	"fmt"
	"time"
)

type APIServer struct {
	crawler *Crawler
	addr    string
}

func NewAPIServer(addr string, crawler *Crawler) *APIServer {
	return &APIServer{crawler, addr}
}

func (a *APIServer) serve() {
	http.HandleFunc("/useragents", a.handleUserAgents)
	http.HandleFunc("/peers", a.handlePeers)
	http.HandleFunc("/count", a.handleCount)
	http.ListenAndServe(a.addr, nil)
}

func (a *APIServer) handlePeers(w http.ResponseWriter, r *http.Request) {
	a.crawler.lock.RLock()
	defer a.crawler.lock.RUnlock()
	only := r.URL.Query().Get("only")
	var peers []string
	for peer, nd := range a.crawler.theList {
		if only == "" {
			peers = append(peers, peer)
			continue
		}
		nodeType, err := GetNodeType(nd.peerInfo.Addrs)
		if err != nil {
			continue
		}
		switch {
		case only == "tor" && nodeType == TorOnly:
			peers = append(peers, peer)
		case only == "dualstack" && nodeType == DualStack:
			peers = append(peers, peer)
		case only == "clearnet" && nodeType == Clearnet:
			peers = append(peers, peer)
		}
	}
	out, err := json.MarshalIndent(peers, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
	}
	if string(out) == "null" {
		out = []byte("[]")
	}
	fmt.Fprint(w, string(out))
}

func (a *APIServer) handleCount(w http.ResponseWriter, r *http.Request) {
	lastActive := r.URL.Query().Get("lastActive")
	only := r.URL.Query().Get("only")
	var d time.Duration
	var err error
	if lastActive != "" {
		d, err = time.ParseDuration(lastActive)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
			return
		}
	}
	count := 0
	for _, nd := range a.crawler.theList {
		if lastActive != "" && nd.lastConnect.Add(d).Before(time.Now()) {
			continue
		}
		if only == "" {
			count++
			continue
		}
		nodeType, err := GetNodeType(nd.peerInfo.Addrs)
		if err != nil {
			continue
		}
		switch {
		case only == "tor" && nodeType == TorOnly:
			count++
		case only == "dualstack" && nodeType == DualStack:
			count++
		case only == "clearnet" && nodeType == Clearnet:
			count++
		}
	}
	fmt.Fprint(w, count)
}

func (a *APIServer) handleUserAgents(w http.ResponseWriter, r *http.Request) {
	ua := make(map[string]int)
	for _, nd := range a.crawler.theList {
		if nd.userAgent != "" {
			count, ok := ua[nd.userAgent]
			if !ok {
				ua[nd.userAgent] = 1
			} else {
				ua[nd.userAgent] = count+1
			}
		}
	}
	out, err := json.MarshalIndent(ua, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
	}
	fmt.Fprint(w, string(out))
}
package main

import (
	"encoding/json"
	"errors"
	ps "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	"gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"io/ioutil"
	"net/http"
	"time"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/jsonpb"
)

type OBClient struct {
	httpClient http.Client
	addr       string
}

func NewOBClient(addr string) *OBClient {
	client := http.Client{Timeout: time.Minute * 3}
	return &OBClient{client, addr}
}

func (c *OBClient) Peers() ([]peer.ID, error) {
	var pids []peer.ID
	url := "http://" + c.addr + "/ob/peers"
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return pids, err
	}
	if resp.StatusCode >= 400 {
		return pids, errors.New(url + " " + resp.Status)
	}

	var peers []string
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&peers)
	if err != nil {
		return pids, err
	}
	for _, p := range peers {
		pid, err := peer.IDB58Decode(p)
		if err != nil {
			return pids, err
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func (c *OBClient) ClosestPeers(peerID peer.ID) ([]peer.ID, error) {
	var pids []peer.ID
	resp, err := c.httpClient.Get("http://" + c.addr + "/ob/closestpeers/" + peerID.Pretty())
	if err != nil {
		return pids, err
	}
	if resp.StatusCode != http.StatusOK {
		return pids, errors.New("Closest peers error")
	}
	var peers []string
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&peers)
	if err != nil {
		return pids, err
	}
	for _, p := range peers {
		pid, err := peer.IDB58Decode(p)
		if err != nil {
			return pids, err
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func (c *OBClient) PeerInfo(peerID peer.ID) (ps.PeerInfo, error) {
	var pi ps.PeerInfo
	resp, err := c.httpClient.Get("http://" + c.addr + "/ob/peerinfo/" + peerID.Pretty())
	if err != nil {
		return pi, err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return pi, err
	}

	if resp.StatusCode != http.StatusOK {
		return pi, errors.New(string(b))
	}

	err = pi.UnmarshalJSON(b)
	if err != nil {
		return pi, err
	}
	return pi, nil
}

func (c *OBClient) UserAgent(peerID peer.ID) (string, error) {
	resp, err := c.httpClient.Get("http://" + c.addr + "/ipns/" + peerID.Pretty() + "/user_agent")
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("User agent not found")
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *OBClient) Profile(peerID peer.ID) (*pb.Profile, error) {
	pro := new(pb.Profile)
	resp, err := c.httpClient.Get("http://" + c.addr + "/ob/profile/" + peerID.Pretty())
	if err != nil {
		return pro, err
	}
	if resp.StatusCode != http.StatusOK {
		return pro, errors.New("Profile not found")
	}

	err = jsonpb.Unmarshal(resp.Body, pro)
	if err != nil {
		return pro, err
	}
	return pro, nil
}

func (c *OBClient) Ping(peerID peer.ID) (bool, error) {
	resp, err := c.httpClient.Get("http://" + c.addr + "/ob/status/" + peerID.Pretty())
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		return false, errors.New("Ping error")
	}
	type status struct {
		Status string `json:"status"`
	}
	var st status
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&st)
	if err != nil {
		return false, err
	}
	return st.Status == "online", nil
}

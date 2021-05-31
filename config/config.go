package config

import (
	"os"

	"github.com/pion/webrtc/v3"
	"gopkg.in/yaml.v2"
)

var Map *ConfMap

type ICEServer struct {
	URLs           []string                 `yaml:"urls"`
	Username       string                   `yaml:"username,omitempty"`
	Credential     interface{}              `yaml:"credential,omitempty"`
	CredentialType webrtc.ICECredentialType `yaml:"credentialType,omitempty"`
}

type PeerConnectionConfigMap struct {
	ICEServers []ICEServer `yaml:"iceServers"`
}

func WebrtcPeerConnectionConfig() webrtc.Configuration {
	cfg := webrtc.Configuration{
		BundlePolicy:  webrtc.BundlePolicyMaxCompat,
		RTCPMuxPolicy: webrtc.RTCPMuxPolicyRequire,
		ICEServers:    make([]webrtc.ICEServer, 0),
	}

	if Map == nil || len(Map.PeerConnectionConfig.ICEServers) == 0 {
		cfg.ICEServers = []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
			{
				URLs:       []string{"turn:coturn.mrjosh.net:3478?transport=udp"},
				Username:   "josh",
				Credential: "12345",
			},
		}
		return cfg
	}

	for _, iceServer := range Map.PeerConnectionConfig.ICEServers {
		cfg.ICEServers = append(cfg.ICEServers, webrtc.ICEServer{
			URLs:           iceServer.URLs,
			Username:       iceServer.Username,
			Credential:     iceServer.Credential,
			CredentialType: iceServer.CredentialType,
		})
	}
	return cfg
}

type ConfMap struct {
	PeerConnectionConfig *PeerConnectionConfigMap `yaml:"peerConnectionConfig"`
}

func LoadFile(filename string) (err error) {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	return yaml.NewDecoder(f).Decode(&Map)
}

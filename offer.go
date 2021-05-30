package main

import (
	"github.com/castyapp/cli/components"

	"github.com/pion/webrtc/v3"
)

func CreateOffer(peerConnection *webrtc.PeerConnection) (string, error) {

	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return "", err
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	if err := peerConnection.SetLocalDescription(offer); err != nil {
		return "", err
	}

	<-gatherComplete

	return components.Encode(offer), nil
}

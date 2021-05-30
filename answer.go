package main

import (
	"log"

	"github.com/castyapp/cli/components"

	"github.com/pion/webrtc/v3"
)

func CreateAnswer(peerConnection *webrtc.PeerConnection) string {

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		log.Fatal(err)
	}

	if err := peerConnection.SetLocalDescription(answer); err != nil {
		log.Fatal(err)
	}

	return components.Encode(answer)
}

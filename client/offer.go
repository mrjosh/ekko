package main

import (
	"log"
	"video/components"

	"github.com/pion/webrtc/v2"
)

func CreateOffer(peerConnection *webrtc.PeerConnection, mediaEngine webrtc.MediaEngine) string {

	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	//if err := mediaEngine.PopulateFromSDP(offer); err != nil {
	//log.Fatal(err)
	//}

	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		log.Fatal(err)
	}

	return components.Encode(offer)
}

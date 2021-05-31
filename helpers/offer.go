package helpers

import (
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

	return EncodeBase64(offer)
}

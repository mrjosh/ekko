package helpers

import (
	"github.com/pion/webrtc/v3"
)

func CreateAnswer(peerConnection *webrtc.PeerConnection) (string, error) {

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return "", err
	}

	if err := peerConnection.SetLocalDescription(answer); err != nil {
		return "", err
	}

	return EncodeBase64(answer)
}

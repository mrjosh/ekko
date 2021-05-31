package hub

import (
	"fmt"
	"net"

	"github.com/pion/webrtc/v3"
)

type User struct {
	Id            string                      `json:"id,omitempty"`
	peer          *webrtc.PeerConnection      `json:"-"`
	track         *webrtc.TrackLocalStaticRTP `json:"-"`
	gatewayConn   net.Conn                    `json:"-"`
	creatingOffer bool                        `json:"-"`
}

func (u *User) CreateLocalTrack(room *Room) error {
	outputTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{
		MimeType:  webrtc.MimeTypeOpus,
		ClockRate: 48000,
		Channels:  2,
	}, fmt.Sprintf("%s", u.Id), fmt.Sprintf("audio_user_%s", u.Id))
	if err != nil {
		return err
	}
	u.track = outputTrack
	room.tracks.Set(u.Id, outputTrack)
	return nil
}

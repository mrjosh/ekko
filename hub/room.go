package hub

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/castyapp/cli/helpers"
	"github.com/gobwas/ws/wsutil"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

type Room struct {
	id     string
	users  cmap.ConcurrentMap
	tracks cmap.ConcurrentMap
}

type JoinRoomRequest struct {
	Offer  string
	RoomId string
	UserId string
}

func (r *Room) FindUser(userId string) (*User, error) {
	cmapUser, exist := r.users.Get(userId)
	if !exist {
		log.Println("User not exists!")
		return nil, fmt.Errorf("Could not find user!")
	}
	user, ok := cmapUser.(*User)
	if !ok {
		return nil, fmt.Errorf("CmapUser not equels to *User type")
	}
	return user, nil
}

func (r *Room) AddUsersTracks(user *User) error {

	defer r.dispatchKeyFrame()

	for userId, userTrack := range r.tracks.Items() {

		track, ok := userTrack.(webrtc.TrackLocal)
		if !ok {
			return fmt.Errorf("CmapUser type is not *User!")
		}

		if user.Id != userId {
			log.Println(fmt.Sprintf("Adding user [%s]", user.Id))
			if _, err := user.peer.AddTransceiverFromTrack(track); err != nil {
				return err
			}
		}

	}

	return nil
}

func (r *Room) SendPeerToAllUsers(userPeer *User) error {

	for _, cmapUser := range r.users.Items() {

		user, ok := cmapUser.(*User)
		if !ok {
			return fmt.Errorf("CmapUser type is not *User!")
		}

		if user.Id != userPeer.Id {

			log.Println(fmt.Sprintf("Adding user [%s] to user [%s]", userPeer.Id, user.Id))
			if _, err := user.peer.AddTransceiverFromTrack(userPeer.track); err != nil {
				log.Println("Err on adding track: ", err)
			}

			newOffer, err := user.peer.CreateOffer(nil)
			if err != nil {
				log.Println("Err on negotiontion offer : ", err)
			}

			if err := user.peer.SetLocalDescription(newOffer); err != nil {
				log.Println("Err on negotiontion set local : ", err)
			}

			offerbase64, err := helpers.EncodeBase64(newOffer)
			if err != nil {
				log.Println("Err on encoding offer : ", err)
			}

			packetData, _ := json.Marshal(&Packet{
				Type: "negotiationneeded",
				Data: offerbase64,
			})

			if err := wsutil.WriteServerText(user.gatewayConn, packetData); err != nil {
				log.Fatal(err)
			}

		}
	}

	return nil
}

func NewRoom(join *JoinRoomRequest) *Room {
	return &Room{
		id:     join.RoomId,
		users:  cmap.New(),
		tracks: cmap.New(),
	}
}

func (r *Room) dispatchKeyFrame() error {

	for _, userPeer := range r.users.Items() {

		user, ok := userPeer.(*User)
		if !ok {
			return fmt.Errorf("CmapUser type is not *User!")
		}

		for _, receiver := range user.peer.GetReceivers() {
			if receiver.Track() == nil {
				continue
			}

			_ = user.peer.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{
					MediaSSRC: uint32(receiver.Track().SSRC()),
				},
			})
		}

	}

	return nil
}

func (r *Room) UserJoined(u *User) {
	r.users.Set(u.Id, u)
	//r.dispatchKeyFrame()
}

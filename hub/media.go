package hub

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/castyapp/cli/helpers"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v3"
)

type Hub struct {
	rooms     cmap.ConcurrentMap
	webrtcApi *webrtc.API
}

func (h *Hub) FindRoom(roomId string) (*Room, error) {
	cmapRoom, ok := h.rooms.Get(roomId)
	if !ok {
		return nil, fmt.Errorf("Room not exists!")
	}
	room, exists := cmapRoom.(*Room)
	if !exists {
		return nil, fmt.Errorf("Room not exists!")
	}
	return room, nil
}

func NewHub() *Hub {
	return &Hub{
		rooms: cmap.New(),
	}
}

func NewSingPortWebrtcHub(listener *net.UDPConn) (*Hub, error) {

	// Enable Extension Headers needed for Simulcast
	m := &webrtc.MediaEngine{}
	//exts := []string{
	//"urn:ietf:params:rtp-hdrext:sdes:mid",
	//"urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id",
	//"urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id",
	//}

	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	//for _, extension := range exts {
	//if err := m.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: extension}, webrtc.RTPCodecTypeAudio); err != nil {
	//return nil, err
	//}
	//}

	// Create a InterceptorRegistry. This is the user configurable RTP/RTCP Pipeline.
	// This provides NACKs, RTCP Reports and other features. If you use `webrtc.NewPeerConnection`
	// this is enabled by default. If you are manually managing You MUST create a InterceptorRegistry
	// for each PeerConnection.
	i := &interceptor.Registry{}

	// Use the default set of Interceptors
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, err
	}

	// Create a SettingEngine, this allows non-standard WebRTC behavior
	settingEngine := webrtc.SettingEngine{}

	// Configure our SettingEngine to use our UDPMux. By default a PeerConnection has
	// no global state. The API+SettingEngine allows the user to share state between them.
	// In this case we are sharing our listening port across many.
	settingEngine.SetICEUDPMux(webrtc.NewICEUDPMux(nil, listener))

	return &Hub{
		rooms: cmap.New(),
		webrtcApi: webrtc.NewAPI(
			webrtc.WithSettingEngine(settingEngine),
			webrtc.WithMediaEngine(m),
			webrtc.WithInterceptorRegistry(i),
		),
	}, nil
}

func (h *Hub) CreateOrFindRoom(join *JoinRoomRequest) *Room {
	var room *Room
	cmapRoom, ok := h.rooms.Get(join.RoomId)
	if ok {
		room = cmapRoom.(*Room)
	} else {
		room = NewRoom(join)
		h.rooms.Set(room.id, room)
	}
	return room
}

func (h *Hub) CreateOrJoinRoom(conn net.Conn, join *JoinRoomRequest) (string, error) {

	// Prepare the configuration
	config := webrtc.Configuration{
		BundlePolicy:  webrtc.BundlePolicyMaxCompat,
		RTCPMuxPolicy: webrtc.RTCPMuxPolicyRequire,
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	room := h.CreateOrFindRoom(join)

	// Create a new RTCPeerConnection
	peerConnection, err := h.webrtcApi.NewPeerConnection(config)
	if err != nil {
		return "", err
	}

	userPeer := &User{
		Id:          join.UserId,
		peer:        peerConnection,
		gatewayConn: conn,
	}

	if err := userPeer.CreateLocalTrack(room); err != nil {
		return "", err
	}
	room.UserJoined(userPeer)

	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {

		log.Println("Got track")

		for {
			// Read RTP packets being sent to Pion
			packet, _, readErr := track.ReadRTP()
			if readErr != nil {
				log.Println(readErr)
				return
			}

			if writeErr := userPeer.track.WriteRTP(packet); writeErr != nil && !errors.Is(writeErr, io.ErrClosedPipe) {
				log.Println(writeErr)
				return
			}
		}

	})

	//if err := room.signalPeerConnections(userPeer); err != nil {
	//log.Fatalln(err)
	//}

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())

		switch connectionState {
		case webrtc.ICEConnectionStateConnected:
			go room.SendPeerToAllUsers(userPeer)
			//if err := room.signalPeerConnections(userPeer); err != nil {
			//log.Println("AddUsersTracks Err: ", err)
			//}
			break
		case webrtc.ICEConnectionStateClosed:
			//if err := room.signalPeerConnections(userPeer); err != nil {
			//log.Fatalln(err)
			//}
		}

	})

	peerConnection.OnNegotiationNeeded(func() {

		log.Println(fmt.Sprintf("Negotiation Needed [%s]", userPeer.Id))

		//newOffer, err := peerConnection.CreateOffer(nil)
		//if err != nil {
		//log.Println("Err on negotiontion offer : ", err)
		//}

		//log.Println("Nego Offer: ", newOffer)

		//if peerConnection.SignalingState() != webrtc.SignalingStateStable {
		//return
		//}

		//offerbase64, err := helpers.Encode(newOffer)
		//if err != nil {
		//log.Println("Err on encoding offer : ", err)
		//}

		//packetData, _ := json.Marshal(&Packet{
		//Type: "negotiationneeded",
		//Data: offerbase64,
		//})

		//if err := wsutil.WriteServerText(userPeer.gatewayConn, packetData); err != nil {
		//log.Fatal(err)
		//}

	})

	if err := room.AddUsersTracks(userPeer); err != nil {
		return "", err
	}

	offer := webrtc.SessionDescription{}
	if err := helpers.DecodeBase64(join.Offer, &offer); err != nil {
		return "", err
	}

	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		return "", err
	}

	// Create an answer
	answer, err := peerConnection.CreateAnswer(&webrtc.AnswerOptions{
		OfferAnswerOptions: webrtc.OfferAnswerOptions{
			VoiceActivityDetection: true,
		},
	})
	if err != nil {
		return "", err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err := peerConnection.SetLocalDescription(answer); err != nil {
		return "", err
	}

	answerChan := make(chan *webrtc.SessionDescription)
	peerConnection.OnICEGatheringStateChange(func(is webrtc.ICEGathererState) {
		if is.String() == "complete" {
			answerChan <- peerConnection.LocalDescription()
		}
	})

	description := <-answerChan

	log.Println("DESK : ", description)

	// Output the answer in base64 so we can paste it in browser
	answerBase64, err := helpers.EncodeBase64(description)
	if err != nil {
		return "", err
	}

	return answerBase64, nil
	//return "", nil
}

func (h *Hub) HTTPHandlerFindRoom(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"success": false, "message": "RoomId required"}`))
		return
	}
	h.CreateOrFindRoom(&JoinRoomRequest{RoomId: roomID})
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true}`))
	return
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Upgrade connection to websocket
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("New Client")

	for {

		data, _, err := wsutil.ReadClientData(conn)
		if err != nil {
			log.Println(err)
			return
		}

		if data != nil {

			packet := new(Packet)
			if err := json.Unmarshal(data, packet); err != nil {
				return
			}

			switch packet.Type {

			case "relaySessionDescription":

				room, err := h.FindRoom(packet.RoomId)
				if err != nil {
					log.Println("Room not exists!")
					return
				}

				user, err := room.FindUser(packet.UserId)
				if err != nil {
					log.Println("User not exists!")
					return
				}

				var remoteDesc webrtc.SessionDescription
				helpers.DecodeBase64(packet.Data.(string), &remoteDesc)

				if err := user.peer.SetRemoteDescription(remoteDesc); err != nil {
					log.Println(err)
				}

				answer, err := helpers.EncodeBase64(user.peer.LocalDescription())
				if err != nil {
					log.Println(err)
				}

				packetData, _ := json.Marshal(&Packet{
					Type: "answer",
					Data: answer,
				})
				if err := wsutil.WriteServerText(conn, packetData); err != nil {
					log.Fatal(err)
				}
			case "negotiationneeded":

				room, err := h.FindRoom(packet.RoomId)
				if err != nil {
					log.Println("Room not exists!")
					return
				}

				user, err := room.FindUser(packet.UserId)
				if err != nil {
					log.Println("User not exists!")
					return
				}

				sd := webrtc.SessionDescription{}
				if err := helpers.DecodeBase64(packet.Data.(string), &sd); err != nil {
					log.Println(err)
				}

				if err := user.peer.SetRemoteDescription(sd); err != nil {
					log.Println(err)
				}

				break

			case "join":

				response := &JoinRoomRequest{
					Offer:  packet.Data.(string),
					RoomId: packet.RoomId,
					UserId: packet.UserId,
				}

				sdpAnswer, err := h.CreateOrJoinRoom(conn, response)
				if err != nil {
					log.Fatal(err)
				}
				packetData, _ := json.Marshal(&Packet{
					Type: "answer",
					Data: sdpAnswer,
				})
				if err := wsutil.WriteServerText(conn, packetData); err != nil {
					log.Fatal(err)
				}
				break

			case "relayICECandidate":

				room, err := h.FindRoom(packet.RoomId)
				if err != nil {
					log.Println("Room not exists!")
					return
				}

				user, err := room.FindUser(packet.UserId)
				if err != nil {
					log.Println("User not exists!")
					return
				}

				candidate := packet.Data.(map[string]interface{})
				sdpMLineIndex := uint16(candidate["sdpMLineIndex"].(float64))
				iceCandidate := webrtc.ICECandidateInit{
					Candidate:     candidate["candidate"].(string),
					SDPMLineIndex: &sdpMLineIndex,
				}

				if err := user.peer.AddICECandidate(iceCandidate); err != nil {
					log.Println("Err on adding iceCandidate: ", err)
				}

				break

			}

		}

	}

}

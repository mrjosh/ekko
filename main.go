package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/castyapp/cli/components"
	"github.com/fatih/color"

	"github.com/gordonklaus/portaudio"
	"github.com/gorilla/websocket"
	opusAudio "github.com/mrjosh/opus"
	"github.com/pion/interceptor"
	"github.com/pion/mediadevices"
	opusCodec "github.com/pion/mediadevices/pkg/codec/opus"
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/webrtc/v3"
)

const gatewayURI = "gateway.mrjosh.net"

func main() {

	log.SetFlags(0)

	username, err := NewInput("username", "Enter your nickname: ", true)
	if err != nil {
		log.Fatal(err)
	}

	room, err := NewInput("room", "Enter a room name: ", true)
	if err != nil {
		log.Fatal(err)
	}

	response, err := http.Get(fmt.Sprintf("https://%s/room.json?room_id=%s", gatewayURI, room))
	if err != nil {
		log.Fatal(err)
	}

	if response.StatusCode != 200 {
		log.Fatal(err)
	}

	client, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("wss://%s", gatewayURI), nil)
	if err != nil {
		log.Fatal(err)
	}

	mediaEngine, codecSelector, err := newMediaEngine()
	if err != nil {
		log.Fatal(err)
	}

	peerConnectionFactory, err := newPeerConnectionFactory(mediaEngine)
	if err != nil {
		log.Fatal(err)
	}

	// PeerConnection Configuration
	var peerConnectionConfig = webrtc.Configuration{
		BundlePolicy:  webrtc.BundlePolicyMaxCompat,
		RTCPMuxPolicy: webrtc.RTCPMuxPolicyRequire,
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
			{
				URLs:       []string{"turn:78.47.129.161:3478?transport=udp"},
				Credential: "12345",
				Username:   "josh",
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := peerConnectionFactory.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		log.Fatal(err)
	}

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Println(fmt.Sprintf("[%s] Connection State Changed: %s", color.CyanString("PeerConnection"), connectionState.String()))
	})

	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i != nil {

			log.Println(fmt.Sprintf("[%s] New Ice Candidate [%s]", color.CyanString("PeerConnection"), i.ToJSON().Candidate))

			//client.WriteJSON(map[string]interface{}{
			//"type":    "relayICECandidate",
			//"data":    i.ToJSON(),
			//"user_id": username,
			//"room_id": room,
			//})

		}
	})

	stream, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		//Video: func(c *mediadevices.MediaTrackConstraints) {
		//c.FrameFormat = prop.FrameFormat(frame.FormatYUY2)
		//c.Width = prop.Int(640)
		//c.Height = prop.Int(480)
		//},
		Audio: func(c *mediadevices.MediaTrackConstraints) {
			c.SampleRate = prop.Int(48000)
		},
		Codec: codecSelector,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, track := range stream.GetTracks() {

		track.OnEnded(func(err error) {
			fmt.Printf("Track (ID: %s) ended with error: %v\n",
				track.ID(), err)
		})

		_, err = peerConnection.AddTransceiverFromTrack(track, webrtc.RtpTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendonly,
		})

		if err != nil {
			log.Fatal(err)
		}

	}

	peerConnection.OnTrack(onPeerConnectionTrack)

	offer, err := CreateOffer(peerConnection)
	if err != nil {
		log.Fatalln(err)
	}

	client.WriteJSON(map[string]interface{}{
		"type":    "join",
		"data":    offer,
		"user_id": username,
		"room_id": room,
	})

	for {

		_, data, err := client.ReadMessage()
		if err != nil {
			//log.Fatal(err)
			select {}
		}

		packet := make(map[string]string)

		if err := json.Unmarshal(data, &packet); err != nil {
			log.Println(err)
			continue
		}

		switch packet["type"] {
		case "icecandidate":
			ice := webrtc.ICECandidateInit{}
			components.Decode(packet["data"], &ice)
			if err := peerConnection.AddICECandidate(ice); err != nil {
				log.Fatal(err)
			}
		case "answer":
			answer := webrtc.SessionDescription{}
			components.Decode(packet["data"], &answer)
			if err := peerConnection.SetRemoteDescription(answer); err != nil {
				log.Fatal(err)
			}
			break
		case "negotiationneeded":

			offer := webrtc.SessionDescription{}
			components.Decode(packet["data"], &offer)

			if err := peerConnection.SetRemoteDescription(offer); err != nil {
				log.Fatal(err)
			}

			answerObj := webrtc.SessionDescription{}
			answer := CreateAnswer(peerConnection)
			components.Decode(answer, &answerObj)

			client.WriteJSON(map[string]string{
				"type":    "relaySessionDescription",
				"data":    answer,
				"user_id": username,
				"room_id": room,
			})
			log.Println(fmt.Sprintf("[%s] Negotiation Needed", color.CyanString("PeerConnection")))
			break
		}

	}

}

func newMediaEngine() (*webrtc.MediaEngine, *mediadevices.CodecSelector, error) {

	opusParams, err := opusCodec.NewParams()
	if err != nil {
		return nil, nil, err
	}

	codecSelector := mediadevices.NewCodecSelector(
		mediadevices.WithAudioEncoders(&opusParams),
		//mediadevices.WithVideoEncoders(&x264Params),
	)

	mediaEngine := &webrtc.MediaEngine{}

	exts := []string{
		"urn:ietf:params:rtp-hdrext:sdes:mid",
		"urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id",
		"urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id",
	}

	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, nil, err
	}

	for _, extension := range exts {
		if err := mediaEngine.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: extension}, webrtc.RTPCodecTypeAudio); err != nil {
			return nil, nil, err
		}
	}

	codecSelector.Populate(mediaEngine)

	return mediaEngine, codecSelector, nil
}

func newPeerConnectionFactory(m *webrtc.MediaEngine) (*webrtc.API, error) {

	i := &interceptor.Registry{}

	//Use the default set of Interceptors
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, err
	}

	return webrtc.NewAPI(
		webrtc.WithMediaEngine(m),
		webrtc.WithInterceptorRegistry(i),
	), nil
}

func onPeerConnectionTrack(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {

	if track.Kind() == webrtc.RTPCodecTypeAudio {

		var (
			sampleRate = int(track.Codec().ClockRate)
			channels   = int(track.Codec().Channels)
		)

		log.Println(fmt.Sprintf("[%s] Received User's Audio Track", color.CyanString("OnTrack")))
		log.Println(fmt.Sprintf("[%s] Track Channels, SampleRate: [%d:%d]", color.CyanString("OnTrack"), channels, sampleRate))

		portaudio.Initialize()
		defer portaudio.Terminate()

		out := make([]int16, 8192)
		stream, err := portaudio.OpenDefaultStream(0, 2, float64(sampleRate), len(out), &out)
		if err != nil {
			log.Fatal(err)
		}

		if err := stream.Start(); err != nil {
			log.Fatal(err)
		}

		s, err := opusAudio.NewDecoder(sampleRate, channels)
		if err != nil {
			log.Fatal(err)
		}

		pcm := make([]int16, 16384)

		for {

			packet, _, err := track.ReadRTP()
			if err != nil {
				log.Fatal(err)
			}

			n, err := s.Decode(packet.Payload, pcm)
			if err != nil {
				log.Fatal(err)
			}

			out = pcm[:n*channels]
			stream.Write()

		}

	}

}

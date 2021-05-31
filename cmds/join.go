package cmds

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/briandowns/spinner"
	"github.com/castyapp/cli/config"
	"github.com/castyapp/cli/helpers"
	"github.com/castyapp/cli/sounds"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/gordonklaus/portaudio"
	"github.com/gorilla/websocket"
	opusAudio "github.com/mrjosh/opus"
	"github.com/pion/interceptor"
	"github.com/pion/mediadevices"
	opusCodec "github.com/pion/mediadevices/pkg/codec/opus"
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/webrtc/v3"

	_ "github.com/pion/mediadevices/pkg/driver/microphone"
)

type Authentication struct {
	Username string
	Roomname string
}

type JoinVoiceChannelFlags struct {
	ConfigFile string
	GatewayURI string
}

func NewJoinVoiceChannelCommand() *cobra.Command {

	log.SetFlags(0)
	//log.SetFlags(log.Lshortfile)

	cFlags := &JoinVoiceChannelFlags{
		GatewayURI: "gateway.mrjosh.net",
	}

	cmd := &cobra.Command{
		Use:   "join",
		Short: "Join a voice channel",
		RunE: func(cmd *cobra.Command, args []string) error {

			username, err := helpers.NewInput("username", "Enter your nickname: ", true)
			if err != nil {
				log.Fatal(err)
			}

			roomname, err := helpers.NewInput("room", "Enter a room name: ", true)
			if err != nil {
				log.Fatal(err)
			}

			auth := &Authentication{
				Username: username,
				Roomname: roomname,
			}

			portaudio.Initialize()
			defer portaudio.Terminate()

			s := spinner.New(spinner.CharSets[26], time.Millisecond*100)
			s.Start()
			s.Prefix = fmt.Sprintf("[%s] Connecting to media server [%s]", color.CyanString("<<-Gateway->>"), cFlags.GatewayURI)

			response, err := http.Get(fmt.Sprintf("https://%s/room.json?room_id=%s", cFlags.GatewayURI, auth.Roomname))
			if err != nil {
				log.Fatal(err)
			}

			if response.StatusCode != 200 {
				log.Fatal(err)
			}

			client, wsResponse, err := websocket.DefaultDialer.Dial(fmt.Sprintf("wss://%s", cFlags.GatewayURI), nil)
			if err != nil {

				s.Stop()

				wsResponseBody, _ := ioutil.ReadAll(wsResponse.Body)

				return errors.New(fmt.Sprintf(
					"[%s] Could not connect to media server. REASON: [%v] BODY: %s",
					color.RedString("ERROR"),
					err,
					wsResponseBody,
				))
			}

			s.Stop()
			log.Println(fmt.Sprintf(
				"[%s] %s [%s]",
				color.CyanString("<<--Gateway->>"),
				color.GreenString("✔️ Connected to media server"),
				cFlags.GatewayURI,
			))

			mediaEngine, codecSelector, err := newMediaEngine()
			if err != nil {
				return err
			}

			peerConnectionFactory, err := newPeerConnectionFactory(mediaEngine)
			if err != nil {
				return err
			}

			// PeerConnection Configuration
			peerConnectionConfig := config.WebrtcPeerConnectionConfig()

			log.Println(fmt.Sprintf(
				"[%s] ICEServers: %s",
				color.CyanString("PeerConnection"),
				peerConnectionConfig.ICEServers,
			))

			// Create a new RTCPeerConnection
			peerConnection, err := peerConnectionFactory.NewPeerConnection(peerConnectionConfig)
			if err != nil {
				return err
			}

			peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
				log.Println(fmt.Sprintf("[%s] Connection State Changed: %s", color.CyanString("PeerConnection"), connectionState.String()))
			})

			var clientJoined *bool = new(bool)
			peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
				if i != nil {

					log.Println(fmt.Sprintf("[%s] New Ice Candidate [%s]", color.CyanString("PeerConnection"), i.ToJSON().Candidate))

					if *clientJoined {
						client.WriteJSON(map[string]interface{}{
							"type":    "relayICECandidate",
							"data":    i.ToJSON(),
							"user_id": username,
							"room_id": auth.Roomname,
						})
					}

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

				if _, err = peerConnection.AddTrack(track); err != nil {
					return err
				}

			}

			peerConnection.OnTrack(onPeerConnectionTrack)

			offer, err := helpers.CreateOffer(peerConnection)
			if err != nil {
				return err
			}

			client.WriteJSON(map[string]interface{}{
				"type":    "join",
				"data":    offer,
				"user_id": auth.Username,
				"room_id": auth.Roomname,
			})

			peerConnection.OnNegotiationNeeded(func() {

				log.Println(fmt.Sprintf("Negotiation Needed"))

				offer, err := helpers.CreateOffer(peerConnection)
				if err != nil {
					log.Fatalln(err)
				}

				client.WriteJSON(map[string]interface{}{
					"type":    "negotiationneeded",
					"data":    offer,
					"user_id": username,
					"room_id": auth.Roomname,
				})

			})

			return handleEvents(client, peerConnection, auth, clientJoined)
		},
	}
	cmd.SuggestionsMinimumDistance = 1
	cmd.Flags().StringVarP(&cFlags.GatewayURI, "gateway-uri", "g", "gateway.mrjosh.net", "Gateway uri")
	cmd.Flags().StringVarP(&cFlags.ConfigFile, "config-file", "c", "", "Casty configuration file")
	return cmd
}

func handleEvents(client *websocket.Conn, peerConnection *webrtc.PeerConnection, auth *Authentication, clientJoined *bool) error {

	go connectionKeepAlive(client)

	for {

		_, data, err := client.ReadMessage()
		if err != nil {
			return err
		}

		packet := make(map[string]string)

		if err := json.Unmarshal(data, &packet); err != nil {
			log.Println(err)
			continue
		}

		switch packet["type"] {
		case "icecandidate":
			ice := webrtc.ICECandidateInit{}
			helpers.DecodeBase64(packet["data"], &ice)
			if err := peerConnection.AddICECandidate(ice); err != nil {
				log.Fatal(err)
			}
			log.Println(fmt.Sprintf("[%s] AddICECandidate", color.CyanString("PeerConnection")))
		case "answer":

			description := webrtc.SessionDescription{}
			helpers.DecodeBase64(packet["data"], &description)

			if err := peerConnection.SetRemoteDescription(description); err != nil {
				log.Fatal(err)
			}

			log.Println(fmt.Sprintf("[%s] [Answer] SetRemoteDescription", color.CyanString("PeerConnection")))

			*clientJoined = true
			break
		case "negotiationneeded":

			log.Println(fmt.Sprintf("[%s] Negotiation Needed", color.CyanString("PeerConnection")))

			offer := webrtc.SessionDescription{}
			helpers.DecodeBase64(packet["data"], &offer)

			if err := peerConnection.SetRemoteDescription(offer); err != nil {
				log.Fatal(err)
			}

			log.Println(fmt.Sprintf("[%s] SetRemoteDescription", color.CyanString("PeerConnection")))

			if offer.Type == webrtc.SDPTypeOffer {

				answerObj := webrtc.SessionDescription{}
				answer, err := helpers.CreateAnswer(peerConnection)
				if err != nil {
					return err
				}

				if err := helpers.DecodeBase64(answer, &answerObj); err != nil {
					return err
				}

				client.WriteJSON(map[string]string{
					"type":    "relaySessionDescription",
					"data":    answer,
					"user_id": auth.Username,
					"room_id": auth.Roomname,
				})
			}

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

	log.Println(fmt.Sprintf(
		"[%s] %s joined the voice channel!",
		color.MagentaString("<<-NewUser->>>"),
		color.GreenString(track.ID()),
	))

	sounds.UserJoined()

	if track.Kind() == webrtc.RTPCodecTypeAudio {

		var (
			sampleRate = int(track.Codec().ClockRate)
			channels   = int(track.Codec().Channels)
		)

		log.Println(fmt.Sprintf(
			"[%s] Playing %s's Audio Track [ Channels:%d , SampleRate: %d]",
			color.MagentaString("NewAudioTrack"),
			color.WhiteString(track.ID()),
			channels,
			sampleRate,
		))

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

func connectionKeepAlive(client *websocket.Conn) {

	ticket := time.NewTicker(time.Second * 10)

	for {
		<-ticket.C
		// Sending ping request
		//log.Println("Sending ping request")
		client.WriteJSON(map[string]string{"type": "ping"})
	}

}

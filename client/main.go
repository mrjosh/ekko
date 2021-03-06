package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"

	"os"
	"strings"
	"video/components"

	"github.com/gorilla/websocket"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec/opus"
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/pkg/media/oggwriter"
)

func NewInput(name, description string, required bool) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(description)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if required {
		if value != "" {
			return strings.TrimSpace(value), nil
		}
		return "", errors.New(name + " is required")
	}
	return strings.TrimSpace(value), nil
}

func main() {

	log.SetFlags(log.Lshortfile | log.Ltime)

	username, err := NewInput("username", "Enter your username: ", true)
	if err != nil {
		log.Fatal(err)
	}

	//enabledMic, err := NewInput("mic", "Enable mic: ", true)
	//if err != nil {
	//log.Fatal(err)
	//}

	peerConnectionConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	//configure codec specific parameters
	//x264Params, _ := x264.NewParams()
	//x264Params.Preset = x264.PresetUltrafast
	//x264Params.BitRate = 1_000_000 // 1mbps

	opusParams, err := opus.NewParams()
	if err != nil {
		panic(err)
	}

	codecSelector := mediadevices.NewCodecSelector(
		mediadevices.WithAudioEncoders(&opusParams),
		//mediadevices.WithVideoEncoders(&x264Params),
	)

	mediaEngine := webrtc.MediaEngine{}
	mediaEngine.RegisterCodec(webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))

	codecSelector.Populate(&mediaEngine)

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	peerConnection, err := api.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		log.Fatal(err)
	}

	//peerConnection, err := webrtc.NewPeerConnection(peerConnectionConfig)
	//if err != nil {
	//log.Fatal(err)
	//}

	//if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
	//panic(err)
	//}

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	//if enabledMic == "y" {
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

	for _, tracker := range stream.GetAudioTracks() {

		tracker.OnEnded(func(err error) {
			fmt.Printf("Track (ID: %s) ended with error: %v\n",
				tracker.ID(), err)
		})

		webrtcTrack, err := tracker.Bind(peerConnection)
		if err != nil {
			panic(err)
		}

		_, err = peerConnection.AddTransceiverFromTrack(webrtcTrack,
			webrtc.RtpTransceiverInit{
				Direction: webrtc.RTPTransceiverDirectionSendrecv,
			},
		)
		if err != nil {
			panic(err)
		}

	}
	//}

	peerConnection.OnDataChannel(func(dataChan *webrtc.DataChannel) {
		log.Println("on data channel called")
		log.Println(dataChan)
	})

	//audioChan := make(chan *rtp.Packet)

	peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {

		log.Println("Received track")

		if track.Kind() == webrtc.RTPCodecTypeAudio {

			tmpFile := fmt.Sprintf("/tmp/%s.ogg", username)
			fmt.Println(fmt.Sprintf("Got Opus track, saving to disk as [%s] (48 kHz, 2 channels)", tmpFile))

			tmpOggFile, err := oggwriter.New(tmpFile, 48000, 2)
			if err != nil {
				panic(err)
			}

			soundCha := make(chan struct{})
			var started = false

			go func() {

				for {

					rtpPacket, err := track.ReadRTP()
					if err != nil {
						log.Fatal(err)
					}

					if err := tmpOggFile.WriteRTP(rtpPacket); err != nil {
						panic(err)
					}

					if !started {
						started = true
						soundCha <- struct{}{}
					}

				}

			}()

			select {
			case <-soundCha:
				execCmd := exec.Command("ffplay", tmpFile, "-nodisp")
				if err := execCmd.Run(); err != nil {
					panic(err)
				}
			}

			//s, err := opusD.NewStream(audio)
			//if err != nil {
			//log.Fatal(err)
			//}

			//defer s.Close()
			//pcmbuf := make([]int16, 16384)

			//for {
			//var buf []int16
			//n, err := s.Read(buf)
			//if err == io.EOF {
			//break
			//} else if err != nil {
			//log.Fatal(err)
			//}
			//pcm := pcmbuf[:n*2]

			//log.Println(pcm)

			//}

			//remaining := int(c.NumSamples)
			//log.Println(remaining)

			//for {
			//err := binary.Read(audio, binary.BigEndian, out)
			//if err == io.EOF {
			//break
			//}
			//if err := stream.Write(); err != nil {
			//log.Fatal(err)
			//}
			//}

			//}()

			//select {
			//case <-soundCha:
			//go func() {
			//for {

			//if err := binary.Read(buffer, binary.LittleEndian, out); err != nil {
			//panic(err)
			//}

			//if err := stream.Write(); err != nil {
			//panic(err)
			//}
			//}
			//}()
			//}

		}

	})

	client, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://127.0.0.1:3000/?u=%s", username), nil)
	if err != nil {
		log.Fatal(err)
	}

	if username == "josh" {
		log.Println("Calling josh2")
		client.WriteJSON(map[string]interface{}{
			"command": "Calling",
			"sdp":     CreateOffer(peerConnection, mediaEngine),
			"to":      "josh2",
		})
	}

	for {

		_, data, err := client.ReadMessage()
		if err != nil {
			log.Fatal(err)
		}

		packet := make(map[string]string)

		if err := json.Unmarshal(data, &packet); err != nil {
			log.Println(err)
			continue
		}

		switch packet["command"] {
		case "Answered":
			answer := webrtc.SessionDescription{}
			components.Decode(packet["sdp"], &answer)
			if err := peerConnection.SetRemoteDescription(answer); err != nil {
				log.Fatal(err)
			}
			break
		case "Calling":

			offer := webrtc.SessionDescription{}
			components.Decode(packet["sdp"], &offer)

			//if err := mediaEngine.PopulateFromSDP(offer); err != nil {
			//log.Fatal(err)
			//}

			if err := peerConnection.SetRemoteDescription(offer); err != nil {
				log.Fatal(err)
			}

			answerObj := webrtc.SessionDescription{}
			answer := CreateAnswer(peerConnection)
			components.Decode(answer, &answerObj)

			if err := mediaEngine.PopulateFromSDP(answerObj); err != nil {
				log.Fatal(err)
			}

			client.WriteJSON(map[string]string{
				"command": "Answered",
				"sdp":     answer,
				"to":      "josh",
			})
			log.Println("Answering...")
			break
		}

		log.Println(packet["command"])

	}

}

type readerAtSeeker interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

type ID [4]byte

func (id ID) String() string {
	return string(id[:])
}

type commonChunk struct {
	NumChans      int16
	NumSamples    int32
	BitsPerSample int16
	SampleRate    [10]byte
}

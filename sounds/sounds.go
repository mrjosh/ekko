package sounds

import (
	"bytes"
	"errors"
	"fmt"
	"log"

	_ "embed"

	"github.com/fatih/color"
	"github.com/gordonklaus/portaudio"
	opusAudio "github.com/mrjosh/opus"
)

const (
	sampleRate = 48000
	channels   = 2
)

var (
	//go:embed user_joined.opus
	userJoinedSound []byte
)

func warn(sound string, err error) {
	log.Println(fmt.Sprintf(
		"[%s] Warning! cannot play {%s} sound! CAUSE: [%v]",
		color.YellowString("PortAudio"),
		fmt.Sprint(sound),
		err,
	))
}

func playSound(sound string) {

	var data []byte

	switch sound {
	case "user_joined":
		data = userJoinedSound
	default:
		warn(sound, errors.New("could not find sound!"))
		return
	}

	go func() {

		out := make([]int16, 8192)

		stream, err := portaudio.OpenDefaultStream(0, 2, float64(sampleRate), len(out), &out)
		if err != nil {
			warn(sound, err)
			return
		}

		if err := stream.Start(); err != nil {
			warn(sound, err)
			return
		}

		defer stream.Close()

		buffer := bytes.NewBuffer(data)

		s, err := opusAudio.NewStream(buffer)
		if err != nil {
			warn(sound, err)
			return
		}

		pcm := make([]int16, 16384)

		for {
			n, err := s.Read(pcm)
			if err != nil {
				return
			}
			out = pcm[:n*channels]
			stream.Write()
		}

	}()

}

func UserJoined() {
	playSound("user_joined")
}

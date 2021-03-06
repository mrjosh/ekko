package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"os"

	"github.com/gordonklaus/portaudio"
	"github.com/mccoyst/ogg"
)

func main() {

	portaudio.Initialize()
	defer portaudio.Terminate()

	//delay := time.Second / 3

	//buffer := make([]float32, int(p.SampleRate*delay.Seconds()))
	//inn := int(0)

	out := make([]int16, 8192)
	stream, err := portaudio.OpenDefaultStream(0, 2, float64(48000), len(out), &out)
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	if err := stream.Start(); err != nil {
		log.Fatal(err)
	}
	defer stream.Stop()

	f, _ := os.Open("../client/josh_voice.ogg")

	decoder := ogg.NewDecoder(f)

	for {

		page, err := decoder.Decode()
		if err != nil {
			panic(err)
		}

		log.Println(page.Packet)

		binary.Read(bytes.NewBuffer(page.Packet), binary.LittleEndian, out)

		if err := stream.Write(); err != nil {
			panic(err)
		}
	}

}

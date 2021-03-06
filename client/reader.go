package main

import (
	"bytes"
	"io"
)

type OpusStreamReader struct {
	rc io.ReadWriter
}

func NewOpusStreamReaderWith(i io.ReadWriter) *OpusStreamReader {
	return &OpusStreamReader{
		rc: i,
	}
}

func NewOpusStreamReader() *OpusStreamReader {
	return &OpusStreamReader{
		rc: new(bytes.Buffer),
	}
}

func (i *OpusStreamReader) Close() error {
	return nil
}

func (i *OpusStreamReader) Read(data []byte) (int, error) {
	return i.rc.Read(data)
}

func (i *OpusStreamReader) Write(data []byte) (int, error) {
	return i.rc.Write(data)
}

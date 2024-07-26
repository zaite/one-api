package requester

import (
	"bufio"
	"bytes"
	"net/http"
)

var StreamClosed = []byte("stream_closed")

type HandlerPrefix[T streamable] func(rawLine *[]byte, dataChan chan T, errChan chan error)

type streamable interface {
	// types.ChatCompletionStreamResponse | types.CompletionResponse
	any
}

type StreamReaderInterface[T streamable] interface {
	Recv() (<-chan T, <-chan error)
	Close()
}

type streamReader[T streamable] struct {
	reader   *bufio.Reader
	response *http.Response
	NoTrim   bool

	handlerPrefix HandlerPrefix[T]

	DataChan chan T
	ErrChan  chan error
}

func (stream *streamReader[T]) Recv() (<-chan T, <-chan error) {
	go stream.processLines()

	return stream.DataChan, stream.ErrChan
}

//nolint:gocognit
func (stream *streamReader[T]) processLines() {
	for {
		rawLine, readErr := stream.reader.ReadBytes('\n')
		if readErr != nil {
			stream.ErrChan <- readErr
			return
		}

		if !stream.NoTrim {
			rawLine = bytes.TrimSpace(rawLine)
			if len(rawLine) == 0 {
				continue
			}
		}

		stream.handlerPrefix(&rawLine, stream.DataChan, stream.ErrChan)

		if rawLine == nil {
			continue
		}

		if bytes.Equal(rawLine, StreamClosed) {
			return
		}
	}
}

func (stream *streamReader[T]) Close() {
	stream.response.Body.Close()
}

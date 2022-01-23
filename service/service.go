package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/jeremija/wl-gammarelay/display"
	"github.com/jeremija/wl-gammarelay/types"
)

type Service struct {
	params          Params
	reqCh           chan requestWithResponse
	display         *display.Display
	lastColorParams display.ColorParams
	listener        net.Listener
}

type Params struct {
	SocketPath  string
	HistoryPath string
}

type requestWithResponse struct {
	request    types.Request
	responseCh chan<- types.Response
}

func New(params Params) *Service {
	return &Service{
		params: params,

		reqCh: make(chan requestWithResponse),

		lastColorParams: display.ColorParams{
			Temperature: 6500,
			Brightness:  1.0,
		},
	}
}

func (s *Service) Listen() error {
	display, err := display.New()
	if err != nil {
		return fmt.Errorf("failed to create display: %w", err)
	}

	s.display = display

	listener, err := net.Listen("unix", s.params.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.listener = listener

	return nil
}

func (s *Service) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.listener.Close()
	}()

	go func() {
		for {
			select {
			case rwr := <-s.reqCh:
				s.handleRequest(rwr.request, rwr.responseCh)
			case <-ctx.Done():
				return
			}
		}
	}()

	conns := map[net.Conn]struct{}{}
	connsMu := sync.Mutex{}

	addConn := func(conn net.Conn) {
		connsMu.Lock()

		conns[conn] = struct{}{}
		log.Printf("New connection. Active connections: %d\n", len(conns))

		connsMu.Unlock()
	}

	removeConn := func(conn net.Conn) {
		connsMu.Lock()

		delete(conns, conn)
		log.Printf("Connection closed. Active connections: %d\n", len(conns))

		connsMu.Unlock()
	}

	closeAllConns := func() {
		connsMu.Lock()
		defer connsMu.Unlock()

		for conn := range conns {
			log.Print("Terminating connection\n")
			conn.Close()
		}
	}

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			closeAllConns()

			return fmt.Errorf("failed to accept conn: %w", err)
		}

		addConn(conn)

		go func(conn net.Conn) {
			defer removeConn(conn)
			s.handleConn(ctx, conn)
		}(conn)
	}
}

func (s *Service) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	read := func() (types.Request, error) {
		var request types.Request

		err := decoder.Decode(&request)

		return request, err
	}

	write := func(response types.Response) bool {
		if err := encoder.Encode(response); err != nil {
			log.Printf("Failed to write response: %s\n", err)
			return false
		}

		return true
	}

	for {
		req, err := read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Client closed the connection.
				return
			}

			log.Printf("Failed to decode message: %s\n", err)

			if !write(types.Response{
				Error: "Failed to decode message",
			}) {
				return
			}

			return
		}

		responseCh := make(chan types.Response, 1)

		select {
		case s.reqCh <- requestWithResponse{
			request:    req,
			responseCh: responseCh,
		}:
		case <-ctx.Done():
			log.Printf("Failed to enqueue request: %s\n", ctx.Err())

			if !write(types.Response{
				Error: "Failed to enqueue request",
			}) {
				return
			}

			return
		}

		select {
		case response := <-responseCh:
			write(response)
		case <-ctx.Done():
			log.Printf("Received no response in time: %s\n", ctx.Err())

			write(types.Response{
				Error: "Received no response in time",
			})

			return
		}
	}
}

// handleRequest will process the request and write to the responseCh. To
// avoid deadlocks, the responseCh must be 1-buffered. Only one message is ever
// going to be written to this channel, and it will be closed at the end.
func (s *Service) handleRequest(request types.Request, responseCh chan<- types.Response) {
	defer close(responseCh)

	requestJSON, _ := json.Marshal(request)

	log.Printf("Handling request: %s\n", string(requestJSON))

	switch {
	case request.Color != nil:
		colorParams, err := newDisplayColorParams(*request.Color, s.lastColorParams)
		if err != nil {
			log.Printf("Failed to create color params: %s\n", err)

			responseCh <- types.Response{
				Error: "Failed to parse parameters",
			}

			return
		}

		err = s.display.SetColor(colorParams)
		if err != nil {
			log.Printf("Failed to set color: %s\n", err)

			responseCh <- types.Response{
				Error: "Failed to set color",
			}

			return
		}

		s.lastColorParams = colorParams

		if err := s.writeHistory(colorParams); err != nil {
			log.Printf("Failed to write history: %s\n", err)
		}

		responseCh <- types.Response{
			Color: &types.Color{
				Temperature: strconv.Itoa(colorParams.Temperature),
				Brightness:  strconv.FormatFloat(float64(colorParams.Brightness), 'f', -1, 32),
			},
		}
	default:
		log.Printf("Unknown request")

		responseCh <- types.Response{
			Error: "Unknown request",
		}
	}
}

func (s *Service) writeHistory(colorParams display.ColorParams) error {
	if s.params.HistoryPath == "" {
		return nil
	}

	history, err := os.OpenFile(s.params.HistoryPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("Failed to open history file: %w", err)
	}

	defer history.Close()

	msg := fmt.Sprintf("%d %f\n", colorParams.Temperature, colorParams.Brightness)

	if _, err := history.Write([]byte(msg)); err != nil {
		return fmt.Errorf("Failed to write history: %w", err)
	}

	return nil
}

func newDisplayColorParams(color types.Color, prev display.ColorParams) (display.ColorParams, error) {
	isRelative := func(str string) bool {
		return strings.HasPrefix(str, "+") || strings.HasPrefix(str, "-")
	}

	ret := display.ColorParams{
		Temperature: prev.Temperature,
		Brightness:  prev.Brightness,
	}

	if color.Temperature != "" {
		temperature, err := strconv.Atoi(color.Temperature)
		if err != nil {
			return display.ColorParams{}, fmt.Errorf("failed to parse temperature: %w", err)
		}

		if isRelative(color.Temperature) {
			ret.Temperature += temperature
		} else {
			ret.Temperature = temperature
		}
	}

	if color.Brightness != "" {
		brightness, err := strconv.ParseFloat(color.Brightness, 32)
		if err != nil {
			return display.ColorParams{}, fmt.Errorf("failed to parse brightness: %w", err)
		}

		if isRelative(color.Brightness) {
			ret.Brightness += float32(brightness)
		} else {
			ret.Brightness = float32(brightness)
		}
	}

	return ret, nil
}

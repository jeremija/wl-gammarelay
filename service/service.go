package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/jeremija/wl-gammarelay/display"
	"github.com/jeremija/wl-gammarelay/types"
)

type Service struct {
	params          Params
	reqCh           chan requestWithResponse
	lastColorParams display.ColorParams
	updatesCh       chan []types.Update
}

type Display interface {
	SetColor(context.Context, display.ColorParams) error
	Close()
}

type Params struct {
	Listener    net.Listener
	Display     Display
	HistoryPath string
	Verbose     bool
}

type requestWithResponse struct {
	conn       *connection
	request    types.Request
	responseCh chan<- types.Response
}

func New(params Params) *Service {
	return &Service{
		params: params,

		reqCh:     make(chan requestWithResponse),
		updatesCh: make(chan []types.Update),

		lastColorParams: display.ColorParams{
			Temperature: 6500,
			Brightness:  1.0,
		},
	}
}

func (s *Service) Close() error {
	s.params.Display.Close()

	return s.params.Listener.Close()
}

func (s *Service) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.params.Listener.Close()
	}()

	go func() {
		for {
			select {
			case rwr := <-s.reqCh:
				s.handleRequest(ctx, rwr)
			case <-ctx.Done():
				return
			}
		}
	}()

	conns := map[*connection]struct{}{}
	connsMu := sync.Mutex{}

	go func() {
		for {
			select {
			case updates := <-s.updatesCh:
				connsMu.Lock()

				allConns := make([]*connection, 0, len(conns))

				for conn := range conns {
					allConns = append(allConns, conn)
				}

				connsMu.Unlock()

				for _, conn := range allConns {
					updatesforConn := make([]types.Update, 0, len(updates))
					for _, update := range updates {

						if conn.IsSubscribed(update.Key) {
							updatesforConn = append(updatesforConn, update)
						}
					}

					if len(updatesforConn) > 0 {
						conn.WriteLogError(types.Response{
							Updates: updatesforConn,
						})
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	addConn := func(conn *connection) {
		connsMu.Lock()

		conns[conn] = struct{}{}

		if s.params.Verbose {
			log.Printf("New connection. Active connections: %d\n", len(conns))
		}

		connsMu.Unlock()
	}

	removeConn := func(conn *connection) {
		connsMu.Lock()

		delete(conns, conn)

		if s.params.Verbose {
			log.Printf("Connection closed. Active connections: %d\n", len(conns))
		}

		connsMu.Unlock()
	}

	closeAllConns := func() {
		connsMu.Lock()
		defer connsMu.Unlock()

		for conn := range conns {
			if s.params.Verbose {
				log.Print("Terminating connection\n")
			}

			conn.Close()
		}
	}

	for {
		netConn, err := s.params.Listener.Accept()
		if err != nil {
			closeAllConns()

			return fmt.Errorf("failed to accept conn: %w", err)
		}

		conn := newConnection(netConn)

		addConn(conn)

		go func(conn *connection) {
			defer removeConn(conn)
			s.handleConn(ctx, conn)
		}(conn)
	}
}

func (s *Service) handleConn(ctx context.Context, conn *connection) {
	defer conn.Close()

	for {
		request, err := conn.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Client closed the connection.
				return
			}

			log.Printf("Failed to decode message: %s\n", err)

			conn.WriteLogError(types.Response{
				Error: "Failed to decode message",
			})

			return
		}

		if s.params.Verbose {
			b, _ := json.Marshal(request)
			log.Printf("Received request: %s\n", string(b))
		}

		responseCh := make(chan types.Response, 1)

		select {
		case s.reqCh <- requestWithResponse{
			request:    request,
			responseCh: responseCh,
			conn:       conn,
		}:
		case <-ctx.Done():
			log.Printf("Failed to enqueue request: %s\n", ctx.Err())

			conn.WriteLogError(types.Response{
				Error: "Failed to enqueue request",
			})

			return
		}

		select {
		case response := <-responseCh:
			if s.params.Verbose {
				b, _ := json.Marshal(response)
				log.Printf("Sending response: %s\n", string(b))
			}

			conn.WriteLogError(response)
		case <-ctx.Done():
			log.Printf("Received no response in time: %s\n", ctx.Err())

			conn.WriteLogError(types.Response{
				Error: "Received no response in time",
			})

			return
		}
	}
}

// handleRequest will process the request and write to the responseCh. To
// avoid deadlocks, the responseCh must be 1-buffered. Only one message is ever
// going to be written to this channel, and it will be closed at the end.
func (s *Service) handleRequest(ctx context.Context, rwr requestWithResponse) {
	request := rwr.request
	responseCh := rwr.responseCh
	conn := rwr.conn

	defer close(rwr.responseCh)

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

		err = s.params.Display.SetColor(ctx, colorParams)
		if err != nil {
			log.Printf("Failed to set color: %s\n", err)

			responseCh <- types.Response{
				Error: "Failed to set color",
			}

			return
		}

		s.lastColorParams = colorParams

		colorResponse := newColor(colorParams)

		responseCh <- types.Response{
			Color: colorResponse,
		}

		updates := []types.Update{
			{
				Key:   types.SubscriptionKeyColor,
				Color: colorResponse,
			},
		}

		// TODO this is blocking so it might slow down the event loop.
		select {
		case s.updatesCh <- updates:
		case <-ctx.Done():
			log.Printf("Failed to broadcast update: %s\n", ctx.Err())
			return
		}
	case request.Subscribe != nil:
		conn.Subscribe(request.Subscribe)

		updates := make([]types.Update, 0, 1)

		// Ensure the current status is sent as a response.
		for _, key := range request.Subscribe {
			switch key {
			case types.SubscriptionKeyColor:
				updates = append(updates, types.Update{
					Key:   key,
					Color: newColor(s.lastColorParams),
				})
			default:
			}
		}

		res := types.Response{
			Subscribed: request.Subscribe,
			Updates:    updates,
		}

		responseCh <- res

	case request.Unsubscribe != nil:
		conn.Unsubscribe(request.Unsubscribe)

		responseCh <- types.Response{
			Unsubscribed: request.Unsubscribe,
		}

	default:
		log.Printf("Unknown request")

		responseCh <- types.Response{
			Error: "Unknown request",
		}
	}
}

func newColor(p display.ColorParams) *types.Color {
	return &types.Color{
		Temperature: strconv.Itoa(p.Temperature),
		Brightness:  strconv.FormatFloat(float64(p.Brightness), 'f', 2, 32),
	}
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

		if ret.Brightness > 1 {
			ret.Brightness = 1
		} else if ret.Brightness < 0 {
			ret.Brightness = 0
		}
	}

	return ret, nil
}

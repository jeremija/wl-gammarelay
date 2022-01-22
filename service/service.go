package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/jeremija/wl-gammarelay/display"
)

type Service struct {
	params          Params
	reqCh           chan requestWithResponse
	display         *display.Display
	lastColorParams display.ColorParams
	listener        net.Listener
	historyWriter   *os.File
}

type Params struct {
	SocketPath  string
	HistoryPath string
}

type requestWithResponse struct {
	request Request
	errCh   chan<- error
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

	if s.params.HistoryPath != "" {
		historyWriter, err := os.OpenFile(s.params.HistoryPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			return fmt.Errorf("cannot open history file: %w", err)
		}

		s.historyWriter = historyWriter
	}

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
				s.processRequest(rwr.request, rwr.errCh)
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept conn: %w", err)
		}

		go func() {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			s.handleConn(ctx, conn)
		}()
	}
}

func (s *Service) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	write := func(res Response) {
		if err := json.NewEncoder(conn).Encode(res); err != nil {
			log.Printf("failed to write message: %s\n", err)
		}
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			log.Printf("Failed to set connection deadline: %s\n", err)
		}
	}

	var req Request

	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		log.Printf("Failed to decode message: %s\n", err)

		write(Response{
			OK:      false,
			Message: "Failed to decode message",
		})

		return
	}

	errCh := make(chan error, 1)

	select {
	case s.reqCh <- requestWithResponse{
		request: req,
		errCh:   errCh,
	}:
	case <-ctx.Done():
		log.Printf("Failed to enqueue request: %s\n", ctx.Err())

		write(Response{
			OK:      false,
			Message: "Failed to enqueue request",
		})

		return
	}

	select {
	case err := <-errCh:
		if err != nil {
			log.Printf("Received error response: %s\n", err)

			write(Response{
				OK:      false,
				Message: err.Error(),
			})
		} else {
			write(Response{
				OK:      false,
				Message: "Success",
			})
		}
	case <-ctx.Done():
		log.Printf("Received no response in time: %s\n", ctx.Err())

		write(Response{
			OK:      false,
			Message: "Received no response in time",
		})

		return
	}
}

func (s *Service) processRequest(request Request, errCh chan<- error) {
	defer close(errCh)

	switch {
	case request.ColorParams != nil:
		colorParams := request.ColorParams.AbsoluteColorParams(s.lastColorParams)

		err := s.display.SetColor(colorParams)
		if err != nil {
			log.Printf("Failed to set color: %s\n", err)

			errCh <- fmt.Errorf("Failed to set color")

			return
		}

		s.lastColorParams = colorParams

		if err := s.writeHistory(colorParams); err != nil {
			log.Printf("Failed to write history: %s\n", err)
		}
	default:
		log.Printf("Unknown request")

		errCh <- fmt.Errorf("Unknown request")
	}
}

func (s *Service) writeHistory(colorParams display.ColorParams) error {
	history := s.historyWriter

	if history == nil {
		return nil
	}

	if err := history.Truncate(0); err != nil {
		return fmt.Errorf("Failed to truncate history file: %w", err)
	}

	if _, err := history.Seek(0, 0); err != nil {
		return fmt.Errorf("Failed to seek to beginning of history file: %w", err)
	}

	msg := fmt.Sprintf("%d %f\n", colorParams.Temperature, colorParams.Brightness)

	if _, err := history.Write([]byte(msg)); err != nil {
		return fmt.Errorf("Failed to write history: %w", err)
	}

	return nil
}

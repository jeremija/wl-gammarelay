package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/jeremija/wl-gammarelay/display"
	"github.com/spf13/pflag"
)

type Arguments struct {
	SocketPath  string
	HistoryPath string

	NoStartDaemon bool

	Temperature string
	Brightness  string
}

func (a Arguments) ColorParams() (ColorParams, error) {
	tempStr := a.Temperature
	brightnessStr := a.Brightness

	isRelative := func(str string) bool {
		return strings.HasPrefix(str, "-") || strings.HasPrefix(str, "+")
	}

	temperature, err := strconv.Atoi(tempStr)
	if err != nil {
		return ColorParams{}, fmt.Errorf("parsing temperature: %w", err)
	}

	brightness, err := strconv.ParseFloat(brightnessStr, 32)
	if err != nil {
		return ColorParams{}, fmt.Errorf("parsing brightness: %w", err)
	}

	return ColorParams{
		ColorParams: display.ColorParams{
			Temperature: temperature,
			Brightness:  float32(brightness),
		},
		TemperatureIsRealtive: isRelative(tempStr),
		BrightnessIsRelative:  isRelative(brightnessStr),
	}, nil
}

type Request struct {
	ColorParams *ColorParams `json:"colorParams,omitempty"`
}

func parseArgs(argsSlice []string) (Arguments, error) {
	var args Arguments

	fs := pflag.NewFlagSet(argsSlice[0], pflag.ContinueOnError)

	fs.SetOutput(os.Stdout)

	fs.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage of %s:\n", argsSlice[0])
		fs.PrintDefaults()
	}

	homeDir, _ := os.UserHomeDir()

	defaultHistoryPath := path.Join(homeDir, ".wl-gammarelay.hist")
	defaultSocketPath := path.Join(homeDir, ".wl-gammarelay.sock")

	fs.StringVarP(&args.HistoryPath, "history", "H", defaultHistoryPath, "History file to use")
	fs.StringVarP(&args.SocketPath, "sock", "s", defaultSocketPath, "Unix domain socket path for RPC")

	fs.StringVarP(&args.Temperature, "temperature", "t", "+0", "Color temperature to set, neutral is 6500.")
	fs.StringVarP(&args.Brightness, "brightness", "b", "+0", "Brightness to set, max is 1.0")

	fs.BoolVarP(&args.NoStartDaemon, "no-daemon", "D", false, "Do not start daemon if not running")

	if err := fs.Parse(argsSlice); err != nil {
		return Arguments{}, fmt.Errorf("parsing args: %w", err)
	}

	return args, nil
}

// main is a test function for proof-of-concept.
func main() {
	args, err := parseArgs(os.Args)
	if err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			os.Exit(2)
		}
		panic(err)
	}

	if err := main2(args); err != nil {
		panic(err)
	}
}

func main2(args Arguments) error {
	ctx := context.Background()

	// We need to handle these events so that the listener removes the socket
	// file gracefully, otherwise the daemon might not start successfully next
	// time.
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	_, err := os.Stat(args.SocketPath)
	if err != nil && !args.NoStartDaemon {
		listener, err := net.Listen("unix", args.SocketPath)
		if err != nil {
			return fmt.Errorf("failed to listen: %w", err)
		}

		go func() {
			<-ctx.Done()
			listener.Close()
		}()

		go func() {
			if err := startService(args.HistoryPath, listener); err != nil {
				log.Fatalf("Failed to start service: %s\n", err)
			}
		}()
	} else {
		cancel()
	}

	colorParams, err := args.ColorParams()
	if err != nil {
		return fmt.Errorf("parsing color params: %w", err)
	}

	conn, err := net.Dial("unix", args.SocketPath)
	if err != nil {
		return fmt.Errorf("dial unix socket: %w", err)
	}

	defer conn.Close()

	log.Printf("sending client request: temperature=%q brightness=%q\n", args.Temperature, args.Brightness)

	err = json.NewEncoder(conn).Encode(Request{
		ColorParams: &colorParams,
	})
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	conn.Close()

	<-ctx.Done()

	return nil
}

func startService(historyPath string, l net.Listener) error {
	disp, err := display.New()
	if err != nil {
		return fmt.Errorf("cannot connect to display: %w", err)
	}

	var history io.Writer

	if historyPath != "" {
		f, err := os.OpenFile(historyPath, os.O_WRONLY|os.O_CREATE, 0)
		if err != nil {
			return fmt.Errorf("cannot open history file: %w", err)
		}

		defer f.Close()

		history = f
	}

	reqCh := make(chan Request)
	defer close(reqCh)

	go func() {
		lastColorParams := display.ColorParams{
			Temperature: 6500,
			Brightness:  1.0,
		}

		for req := range reqCh {
			switch {
			case req.ColorParams != nil:
				colorParams := req.ColorParams.AbsoluteColorParams(lastColorParams)
				err := disp.SetColor(colorParams)
				if err != nil {
					log.Printf("Failed to set color: %s\n", err)
					continue
				}

				lastColorParams = colorParams

				if history != nil {
					_, err := history.Write([]byte(fmt.Sprintf("\r%d %f", lastColorParams.Temperature, lastColorParams.Brightness)))
					if err != nil {
						log.Printf("Failed to write history: %v\n", req.ColorParams)
					}
				}
			default:
				log.Printf("Unexpected request\n")
			}
		}
	}()

	handleConn := func(conn net.Conn) {
		log.Printf("new conn\n")
		defer log.Printf("conn closed\n")

		defer conn.Close()

		decoder := json.NewDecoder(conn)

		var req Request

		if err := decoder.Decode(&req); err != nil {
			log.Printf("Failed to unmarshal JSON: %s\n", err)
			return
		}

		reqCh <- req
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept conn: %w", err)
		}

		handleConn(conn)
	}
}

type ColorParams struct {
	display.ColorParams
	TemperatureIsRealtive bool
	BrightnessIsRelative  bool
}

func (s *ColorParams) AbsoluteColorParams(prev display.ColorParams) display.ColorParams {
	p := s.ColorParams

	if s.TemperatureIsRealtive {
		p.Temperature = prev.Temperature + p.Temperature
	}

	if s.BrightnessIsRelative {
		p.Brightness = prev.Brightness + p.Brightness
	}

	return p
}

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
	"syscall"
	"time"

	"github.com/jeremija/wl-gammarelay/display"
	"github.com/jeremija/wl-gammarelay/service"
	"github.com/jeremija/wl-gammarelay/types"
	"github.com/spf13/pflag"
)

var (
	Version    = "unknown"
	CommitHash = ""
)

type Arguments struct {
	SocketPath  string
	HistoryPath string

	NoStartDaemon    bool
	ReconnectTimeout time.Duration

	Temperature string
	Brightness  string

	Subscribe []string

	Version bool
	Verbose bool
}

func (a Arguments) Color() types.Color {
	return types.Color{
		Temperature: a.Temperature,
		Brightness:  a.Brightness,
	}
}

func getSocketDir() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir != "" {
		return runtimeDir
	}

	if homeDir, err := os.UserHomeDir(); err == nil {
		return homeDir
	}

	return ""
}

func parseArgs(argsSlice []string) (Arguments, error) {
	var args Arguments

	fs := pflag.NewFlagSet(argsSlice[0], pflag.ContinueOnError)

	fs.SetOutput(os.Stdout)

	fs.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage of %s:\n", argsSlice[0])
		fs.PrintDefaults()
	}

	defaultSocketPath := path.Join(getSocketDir(), "wl-gammarelay.sock")

	fs.StringVarP(&args.HistoryPath, "history", "H", "", "History file to use")
	fs.StringVarP(&args.SocketPath, "sock", "s", defaultSocketPath, "Unix domain socket path for RPC")

	fs.StringVarP(&args.Temperature, "temperature", "t", "", "Color temperature to set, neutral is 6500.")
	fs.StringVarP(&args.Brightness, "brightness", "b", "", "Brightness to set, max is 1.0")

	fs.BoolVarP(&args.NoStartDaemon, "no-daemon", "D", false, "Do not start daemon if not running")

	fs.StringSliceVarP(&args.Subscribe, "subscribe", "S", nil, "Subscribe to certain updates")
	fs.DurationVarP(&args.ReconnectTimeout, "reconnect-timeout", "T", 5*time.Second, "Time to reconnect on subscribe")

	fs.BoolVarP(&args.Version, "version", "V", false, "Print version and exit")
	fs.BoolVarP(&args.Verbose, "verbose", "v", false, "Print client socket request and response messages")

	if err := fs.Parse(argsSlice); err != nil {
		return Arguments{}, fmt.Errorf("parsing args: %w", err)
	}

	return args, nil
}

func writeRequest(request types.Request) {
	json.NewEncoder(os.Stdout).Encode(request)
}

func writeResponse(response types.Response) {
	json.NewEncoder(os.Stdout).Encode(response)
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
	if args.Version {
		fmt.Println(Version)

		if CommitHash != "" {
			fmt.Println(CommitHash)
		}

		return nil
	}

	ctx := context.Background()

	// We need to handle these events so that the listener removes the socket
	// file gracefully, otherwise the daemon might not start successfully next
	// time.
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM, syscall.SIGPIPE)
	defer cancel()

	var startedDaemon bool

	_, err := os.Stat(args.SocketPath)
	if err != nil && !args.NoStartDaemon {
		service, err := newDaemon(args.SocketPath, args.HistoryPath, args.Verbose)
		if err != nil {
			return fmt.Errorf("failed to start service: %w", err)
		}

		defer service.Close()

		log.Printf("Started daemon\n")

		go service.Serve(ctx)

		startedDaemon = true
	}

	switch {
	case args.Temperature != "" || args.Brightness != "":
		if err := setTemperature(args); err != nil {
			return fmt.Errorf("set temperature failed: %w", err)
		}

	case args.Subscribe != nil:
		if err := subscribe(ctx, args); err != nil {
			if errors.Is(err, ctx.Err()) {
				return nil
			}

			return fmt.Errorf("subscribe failed: %w", err)
		}
	}

	// If we started the server, keep running until the context is canceled, otherwise bail.
	if startedDaemon {
		<-ctx.Done()
	}

	return nil
}

func newDaemon(socketPath string, historyPath string, verbose bool) (*service.Service, error) {
	display, err := display.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create display: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	return service.New(service.Params{
		Listener:    listener,
		Display:     display,
		HistoryPath: historyPath,
		Verbose:     verbose,
	}), nil
}

func setTemperature(args Arguments) error {
	cl, err := dialClient(args.SocketPath, args.Verbose)
	if err != nil {
		return fmt.Errorf("dialling client: %w", err)
	}

	defer cl.Close()

	color := args.Color()

	request := types.Request{
		Color: &color,
	}

	if err := cl.Write(request); err != nil {
		return fmt.Errorf("writing request: %w", err)
	}

	_, err = cl.Read()
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	return nil
}

func subscribe(ctx context.Context, args Arguments) error {
	connectAndReadLoop := func() error {
		cl, err := dialClient(args.SocketPath, args.Verbose)
		if err != nil {
			return fmt.Errorf("dialling client: %w", err)
		}

		defer cl.Close()

		keys := make([]types.SubscriptionKey, len(args.Subscribe))

		for i, key := range args.Subscribe {
			keys[i] = types.SubscriptionKey(key)
		}

		request := types.Request{
			Subscribe: keys,
		}

		if err := cl.Write(request); err != nil {
			return fmt.Errorf("writing request: %w", err)
		}

		go func() {
			<-ctx.Done()

			cl.Close()
		}()

		for {
			response, err := cl.Read()
			if err != nil {
				return fmt.Errorf("read error: %w", err)
			}

			writeResponse(response)
		}
	}

	for {
		// connectAndReadLoop always returns an error when it's done.
		err := connectAndReadLoop()

		// If the cause was context, we're done.
		if ctx.Err() != nil {
			return fmt.Errorf("context done: %w", err)
		}

		if errors.Is(err, io.EOF) {
			continue
		}

		if args.ReconnectTimeout <= 0 {
			return fmt.Errorf("subscribe failed: %w", err)
		}

		log.Printf("Dial failed, reconnecting in: %s\n", args.ReconnectTimeout)

		timer := time.NewTimer(args.ReconnectTimeout)

		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("context done: %w", ctx.Err())
		}
	}
}

type client struct {
	conn net.Conn

	decoder *json.Decoder
	encoder *json.Encoder

	verbose bool
}

func dialClient(socketPath string, verbose bool) (*client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial unix socket: %w", err)
	}

	return &client{
		conn: conn,

		encoder: json.NewEncoder(conn),
		decoder: json.NewDecoder(conn),

		verbose: verbose,
	}, nil
}

func (c *client) Close() error {
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("failed to close: %w", err)
	}

	return nil
}

func (c *client) Write(request types.Request) error {
	if c.verbose {
		writeRequest(request)
	}

	if err := c.encoder.Encode(request); err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	return nil
}

func (c *client) Read() (types.Response, error) {
	var response types.Response

	if err := c.decoder.Decode(&response); err != nil {
		return types.Response{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if c.verbose {
		writeResponse(response)
	}

	return response, nil
}

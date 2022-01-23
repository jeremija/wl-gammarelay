package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"syscall"

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

	NoStartDaemon bool

	Temperature string
	Brightness  string

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

	tempDir := os.TempDir()

	defaultHistoryPath := path.Join(tempDir, ".wl-gammarelay.hist")
	defaultSocketPath := path.Join(getSocketDir(), "wl-gammarelay.sock")

	fs.StringVarP(&args.HistoryPath, "history", "H", defaultHistoryPath, "History file to use")
	fs.StringVarP(&args.SocketPath, "sock", "s", defaultSocketPath, "Unix domain socket path for RPC")

	fs.StringVarP(&args.Temperature, "temperature", "t", "", "Color temperature to set, neutral is 6500.")
	fs.StringVarP(&args.Brightness, "brightness", "b", "", "Brightness to set, max is 1.0")

	fs.BoolVarP(&args.NoStartDaemon, "no-daemon", "D", false, "Do not start daemon if not running")

	fs.BoolVarP(&args.Version, "version", "V", false, "Print version and exit")
	fs.BoolVarP(&args.Verbose, "verbose", "v", false, "Print client socket request and response messages")

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

	if args.Version {
		fmt.Println(Version)

		if CommitHash != "" {
			fmt.Println(CommitHash)
		}

		return
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
		service := service.New(service.Params{
			SocketPath:  args.SocketPath,
			HistoryPath: args.HistoryPath,
			Verbose:     args.Verbose,
		})

		if err := service.Listen(); err != nil {
			return fmt.Errorf("failed to start service: %w", err)
		}

		log.Printf("Started daemon\n")

		go func() {
			if err := service.Serve(ctx); err != nil {
				log.Printf("Serve done: %s\n", err)
			}
		}()
	} else {
		// So we don't block at the end.
		cancel()
	}

	// Act as a client.
	color := args.Color()

	conn, err := net.Dial("unix", args.SocketPath)
	if err != nil {
		return fmt.Errorf("dial unix socket: %w", err)
	}

	defer conn.Close()

	request, err := json.Marshal(types.Request{
		Color: &color,
	})
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	if args.Verbose {
		fmt.Println(string(request))
	}

	if err = json.NewEncoder(conn).Encode(json.RawMessage(request)); err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	var res json.RawMessage

	if err := json.NewDecoder(conn).Decode(&res); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if args.Verbose {
		fmt.Println(string(res))
	}

	conn.Close()

	// If we started the server, keep running until the context is canceled.
	<-ctx.Done()

	return nil
}

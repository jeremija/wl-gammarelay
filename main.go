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
	"strconv"
	"strings"
	"syscall"

	"github.com/jeremija/wl-gammarelay/display"
	"github.com/jeremija/wl-gammarelay/service"
	"github.com/spf13/pflag"
)

type Arguments struct {
	SocketPath  string
	HistoryPath string

	NoStartDaemon bool

	Temperature string
	Brightness  string
}

func (a Arguments) ColorParams() (service.ColorParams, error) {
	tempStr := a.Temperature
	brightnessStr := a.Brightness

	isRelative := func(str string) bool {
		return strings.HasPrefix(str, "-") || strings.HasPrefix(str, "+")
	}

	temperature, err := strconv.Atoi(tempStr)
	if err != nil {
		return service.ColorParams{}, fmt.Errorf("parsing temperature: %w", err)
	}

	brightness, err := strconv.ParseFloat(brightnessStr, 32)
	if err != nil {
		return service.ColorParams{}, fmt.Errorf("parsing brightness: %w", err)
	}

	return service.ColorParams{
		ColorParams: display.ColorParams{
			Temperature: temperature,
			Brightness:  float32(brightness),
		},
		TemperatureIsRealtive: isRelative(tempStr),
		BrightnessIsRelative:  isRelative(brightnessStr),
	}, nil
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
		service := service.New(service.Params{
			SocketPath:  args.SocketPath,
			HistoryPath: args.HistoryPath,
		})

		if err := service.Listen(); err != nil {
			return fmt.Errorf("failed to start service: %w", err)
		}

		log.Printf("Started daemon\n")

		go func() {
			if err := service.Serve(ctx); err != nil {
				log.Printf("Failed to serve: %s\n", err)
			}
		}()
	} else {
		// So we don't block at the end.
		cancel()
	}

	log.Printf("Starting client\n")

	// Act as a client.
	colorParams, err := args.ColorParams()
	if err != nil {
		return fmt.Errorf("parsing color params: %w", err)
	}

	conn, err := net.Dial("unix", args.SocketPath)
	if err != nil {
		return fmt.Errorf("dial unix socket: %w", err)
	}

	defer conn.Close()

	err = json.NewEncoder(conn).Encode(service.Request{
		ColorParams: &colorParams,
	})
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	var res service.Response

	if err := json.NewDecoder(conn).Decode(&res); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	fmt.Println(res.Message)

	conn.Close()

	// If we started the server, keep running until the context is canceled.
	<-ctx.Done()

	return nil
}

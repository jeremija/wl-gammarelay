package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jeremija/wl-gammarelay/display"
	"github.com/peer-calls/log"
	"github.com/spf13/pflag"
)

var (
	Version    = "unknown"
	CommitHash = ""
)

type Arguments struct {
	Version   bool
	Verbose   bool
	Subscribe []string
}

func parseArgs(argsSlice []string) (Arguments, error) {
	var args Arguments

	fs := pflag.NewFlagSet(argsSlice[0], pflag.ContinueOnError)

	fs.SetOutput(os.Stdout)

	fs.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage of %s:\n", argsSlice[0])
		fs.PrintDefaults()
	}

	fs.StringSliceVarP(&args.Subscribe, "subscribe", "s", nil,
		"Don't start the server, subscribe to changes. Allowed values: Temperature, Brightness")
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
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	if err := main2(args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
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

	logger := log.New().WithNamespace("wl-gammarelay")
	level := log.LevelInfo
	if args.Verbose {
		level = log.LevelTrace
	}

	logger = logger.WithConfig(log.NewConfig(log.ConfigMap{
		"wl-gammarelay":    level,
		"wl-gammarelay:**": level,
	}))

	ctx := context.Background()

	// We need to handle these events so that the listener removes the socket
	// file gracefully, otherwise the daemon might not start successfully next
	// time.
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM, syscall.SIGPIPE)
	defer cancel()

	if len(args.Subscribe) > 0 {
		return NewSubscriber(ctx, args.Subscribe)
	}

	conn, err := NewDBus(logger)
	if err != nil {
		return fmt.Errorf("failed to connect to dbus: %w", err)
	}

	defer conn.Close()

	disp, err := display.New(logger)
	if err != nil {
		return fmt.Errorf("failed to open display: %w", err)
	}

	defer disp.Close()

	if err := conn.RegisterDisplayService(ctx, disp); err != nil {
		return fmt.Errorf("failed to initialize display service")
	}

	<-ctx.Done()

	return nil
}

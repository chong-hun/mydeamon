package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/chenxian/learning-go-daemon/internal/app"
	"github.com/chenxian/learning-go-daemon/internal/state"
)

type parsedArgs struct {
	command    string
	foreground bool
}

func parseArgs(args []string) (parsedArgs, error) {
	parsed := parsedArgs{command: "start"}
	commandSet := false

	for _, arg := range args {
		switch arg {
		case "start", "stop", "status", "logs":
			if commandSet {
				return parsedArgs{}, errors.New("multiple commands provided")
			}
			parsed.command = arg
			commandSet = true
		case "--foreground":
			parsed.foreground = true
		default:
			return parsedArgs{}, fmt.Errorf("unknown argument: %s", arg)
		}
	}

	return parsed, nil
}

func main() {
	parsed, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	stateDir, err := state.DefaultStateDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cfg := app.DefaultConfig(stateDir)
	cfg.Foreground = parsed.foreground

	switch parsed.command {
	case "start":
		logFile, err := state.OpenLogFile(state.LogPath(stateDir))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer logFile.Close()

		logger := log.New(logFile, "", log.LstdFlags)
		daemon := app.New(cfg, logger)
		if err := daemon.Start([]string{"start"}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "stop":
		daemon := app.New(cfg, log.New(os.Stderr, "", 0))
		if err := daemon.Stop(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "status":
		daemon := app.New(cfg, log.New(os.Stderr, "", 0))
		status, err := daemon.Status()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(status)
	case "logs":
		fmt.Println(state.LogPath(stateDir))
	default:
		fmt.Fprintln(os.Stderr, "unknown command")
		os.Exit(1)
	}
}

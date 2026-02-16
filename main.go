package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"log/slog"

	"github.com/redhatinsights/rhc-worker-playbook/internal/config"
	"github.com/redhatinsights/rhc-worker-playbook/internal/constants"
	"github.com/redhatinsights/yggdrasil/worker"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

func main() {
	app := cli.NewApp()
	app.Name = "rhc-worker-playbook"
	app.Version = constants.Version
	app.Usage = "yggdrasil worker that receives and runs ansible playbooks"

	defaultConfigFilePath := filepath.Join(constants.ConfigDir, "rhc-worker-playbook.toml")

	app.Flags = []cli.Flag{
		&cli.PathFlag{
			Name:      "config",
			Value:     defaultConfigFilePath,
			TakesFile: true,
			Usage:     "path to `FILE` containing configuration values (optional)",
		},
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  config.FlagNameDirective,
			Value: config.DefaultConfig.Directive,
			Usage: "set directive to `DIRECTIVE`",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  config.FlagNameLogLevel,
			Value: config.DefaultConfig.LogLevel,
			Usage: "set log level to `LEVEL`",
		}),
		altsrc.NewBoolFlag(&cli.BoolFlag{
			Name:  config.FlagNameVerifyPlaybook,
			Value: config.DefaultConfig.VerifyPlaybook,
			Usage: "use GPG signature verification before executing a playbook",
		}),
		altsrc.NewDurationFlag(&cli.DurationFlag{
			Name:   config.FlagNameResponseInterval,
			Value:  config.DefaultConfig.ResponseInterval,
			Usage:  "override per-message response interval value",
			Hidden: true,
		}),
		altsrc.NewIntFlag(&cli.IntFlag{
			Name:   config.FlagNameBatchEvents,
			Value:  config.DefaultConfig.BatchEvents,
			Usage:  "number of events to batch together in a single transmision",
			Hidden: true,
		}),
	}

	app.Before = beforeAction
	app.Action = mainAction

	if err := app.Run(os.Args); err != nil {
		// previously called log.Fatal, equivalent to print and os.Exit(1)
		// https://pkg.go.dev/log#Fatal
		slog.Error(err.Error())
		os.Exit(1)
	}
}

// beforeAction loads flag values from a config file only if the
// "config" flag value is non-zero.
func beforeAction(ctx *cli.Context) error {
	filePath := ctx.String("config")
	if filePath != "" {
		inputSource, err := altsrc.NewTomlSourceFromFile(filePath)
		if err != nil {
			return cli.Exit(err, 1)
		}
		return altsrc.ApplyInputSourceValues(ctx, inputSource, ctx.App.Flags)
	}
	return nil
}

func mainAction(ctx *cli.Context) error {
	loadConfigFromContext(ctx)
	level, err := parseLevel(config.DefaultConfig.LogLevel)
	if err != nil {
		return cli.Exit(err, 1)
	}
	slog.SetLogLoggerLevel(level)

	w, err := worker.NewWorker(config.DefaultConfig.Directive, true, nil, nil, rx, nil)
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot create worker: %w", err), 1)
	}

	// Set up a channel to receive the TERM or INT signal over and clean up
	// before quitting.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	if err := w.Connect(quit); err != nil {
		return cli.Exit(fmt.Errorf("cannot connect: %w", err), 1)
	}

	return nil
}

// loadConfigFromContext reads values from the context and sets them in the
// global config.DefaultConfig variable.
func loadConfigFromContext(ctx *cli.Context) {
	config.DefaultConfig.Directive = ctx.String(config.FlagNameDirective)
	config.DefaultConfig.LogLevel = ctx.String(config.FlagNameLogLevel)
	config.DefaultConfig.VerifyPlaybook = ctx.Bool(config.FlagNameVerifyPlaybook)
	config.DefaultConfig.ResponseInterval = ctx.Duration(config.FlagNameResponseInterval)
	config.DefaultConfig.BatchEvents = ctx.Int(config.FlagNameBatchEvents)
}

// parseLevel parses the log level string from the config to an slog.Level
func parseLevel(str string) (slog.Level, error) {
	LevelUnknown := slog.Level(-99)

	switch strings.ToUpper(str) {
	case "ERROR":
		return slog.LevelError, nil
	case "WARN":
		return slog.LevelWarn, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "DEBUG":
		return slog.LevelDebug, nil
	case "TRACE":
		// slog has no "trace" level by default, use LevelDebug
		// We've migrated all prior "trace" level log calls to "debug" level,
		// but trace should still be a valid config option.
		return slog.LevelDebug, nil
	}

	return LevelUnknown, fmt.Errorf("cannot parse log level: %v", str)
}

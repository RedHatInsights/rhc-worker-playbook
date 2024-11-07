package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"git.sr.ht/~spc/go-log"
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
		altsrc.NewBoolFlag(&cli.BoolFlag{
			Name:  config.FlagNameInsightsCoreGPGCheck,
			Value: config.DefaultConfig.InsightsCoreGPGCheck,
			Usage: "perform GPG signature verification on insights-core.egg",
		}),
		altsrc.NewDurationFlag(&cli.DurationFlag{
			Name:   config.FlagNameResponseInterval,
			Value:  config.DefaultConfig.ResponseInterval,
			Usage:  "override per-message response interval value",
			Hidden: true,
		}),
	}

	app.Before = beforeAction
	app.Action = mainAction

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
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
	level, err := log.ParseLevel(config.DefaultConfig.LogLevel)
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot unmarshal log-level: %w", err), 1)
	}
	log.SetLevel(level)

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
	config.DefaultConfig.InsightsCoreGPGCheck = ctx.Bool(config.FlagNameInsightsCoreGPGCheck)
	config.DefaultConfig.ResponseInterval = ctx.Duration(config.FlagNameResponseInterval)
}

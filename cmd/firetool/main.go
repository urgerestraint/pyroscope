package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"

	firecontext "github.com/grafana/fire/pkg/fire/context"
)

var cfg struct {
	verbose bool
	blocks  struct {
		path               string
		restoreMissingMeta bool
	}
}

var (
	consoleOutput = os.Stderr
	logger        = log.NewLogfmtLogger(consoleOutput)
)

func main() {
	ctx := firecontext.WithLogger(context.Background(), logger)

	app := kingpin.New(filepath.Base(os.Args[0]), "Tooling for Grafana Fire, the continuous profiling aggregation system.").UsageWriter(os.Stdout)
	app.Version(version.Print("firetool"))
	app.HelpFlag.Short('h')
	app.Flag("verbose", "Enable verbose logging.").Short('v').Default("0").BoolVar(&cfg.verbose)

	blocksCmd := app.Command("blocks", "Operate on Grafana Fire's blocks.")
	blocksCmd.Flag("path", "Path to blocks directory").Default("./data/local").StringVar(&cfg.blocks.path)

	blocksListCmd := blocksCmd.Command("list", "List blocks.")
	blocksListCmd.Flag("restore-missing-meta", "").Default("false").BoolVar(&cfg.blocks.restoreMissingMeta)

	parsedCmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	if !cfg.verbose {
		logger = level.NewFilter(logger, level.AllowWarn())
	}

	switch parsedCmd {
	case blocksListCmd.FullCommand():
		os.Exit(checkError(blocksList(ctx)))
	}
}

func checkError(err error) int {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
package utils

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type Mode string

const (
	ModeAPI                  Mode = "api"
	ModeManage               Mode = "manage"
	ModeProbe                Mode = "probe"
	ModeSeed                 Mode = "seed"
	ModeWorker               Mode = "worker"
	ModeBackfillProbeRollups Mode = "backfill-probe-rollups"
	ModeBackfillProbeDerived Mode = "backfill-probe-derived"
)

type Flags struct {
	Mode Mode

	// mode specific flags
	SeedCount int
	SeedClear bool

	BackfillProbeRollupsStart         time.Time
	BackfillProbeRollupsEnd           time.Time
	BackfillProbeRollupsChunkDuration time.Duration

	BackfillProbeDerivedCutoff           time.Time
	BackfillProbeDerivedServiceBatchSize int
}

var seedFlags = flag.NewFlagSet("seed", flag.ExitOnError)
var backfillProbeRollupsFlags = flag.NewFlagSet("backfill-probe-rollups", flag.ExitOnError)
var backfillProbeDerivedFlags = flag.NewFlagSet("backfill-probe-derived", flag.ExitOnError)

func ParseFlags() Flags {
	if len(os.Args) < 2 {
		return Flags{Mode: ModeAPI}
	}

	mode := Mode(os.Args[1])

	switch mode {
	case ModeAPI, ModeManage, ModeProbe, ModeWorker:
		return Flags{Mode: mode}

	case ModeSeed:
		count := seedFlags.Int("count", 10, "number of services to seed")
		clear := seedFlags.Bool("clear", false, "clear existing data before seeding")
		seedFlags.Parse(os.Args[2:])
		return Flags{Mode: ModeSeed, SeedCount: *count, SeedClear: *clear}

	case ModeBackfillProbeRollups:
		startDefault := "1970-01-01T00:00:00Z"
		endDefault := time.Now().UTC().Truncate(time.Hour).Format(time.RFC3339)
		startRaw := backfillProbeRollupsFlags.String("start", startDefault, "inclusive RFC3339 start time")
		endRaw := backfillProbeRollupsFlags.String("end", endDefault, "exclusive RFC3339 end time")
		chunkDuration := backfillProbeRollupsFlags.Duration("chunk-duration", 24*time.Hour, "whole-hour chunk duration for production-safe backfills; set 0 to disable chunking")
		backfillProbeRollupsFlags.Parse(os.Args[2:])

		start, err := time.Parse(time.RFC3339, *startRaw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --start value %q: %v\n\n", *startRaw, err)
			printUsage()
			os.Exit(1)
		}
		end, err := time.Parse(time.RFC3339, *endRaw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --end value %q: %v\n\n", *endRaw, err)
			printUsage()
			os.Exit(1)
		}
		return Flags{
			Mode:                              ModeBackfillProbeRollups,
			BackfillProbeRollupsStart:         start,
			BackfillProbeRollupsEnd:           end,
			BackfillProbeRollupsChunkDuration: *chunkDuration,
		}

	case ModeBackfillProbeDerived:
		cutoffRaw := backfillProbeDerivedFlags.String("cutoff", "", "required exclusive RFC3339 cutoff time")
		serviceBatchSize := backfillProbeDerivedFlags.Int("service-batch-size", 500, "number of services to process per transaction batch")
		backfillProbeDerivedFlags.Parse(os.Args[2:])

		if *cutoffRaw == "" {
			fmt.Fprintf(os.Stderr, "missing required --cutoff value\n\n")
			printUsage()
			os.Exit(1)
		}
		cutoff, err := time.Parse(time.RFC3339, *cutoffRaw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --cutoff value %q: %v\n\n", *cutoffRaw, err)
			printUsage()
			os.Exit(1)
		}
		if *serviceBatchSize <= 0 {
			fmt.Fprintf(os.Stderr, "invalid --service-batch-size value %d: must be positive\n\n", *serviceBatchSize)
			printUsage()
			os.Exit(1)
		}
		return Flags{
			Mode:                                 ModeBackfillProbeDerived,
			BackfillProbeDerivedCutoff:           cutoff,
			BackfillProbeDerivedServiceBatchSize: *serviceBatchSize,
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
		return Flags{}
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  aretheyup [subcommand] [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  api      Start the HTTP API server (default)\n")
	fmt.Fprintf(os.Stderr, "  manage   Open the service management TUI\n")
	fmt.Fprintf(os.Stderr, "  probe    Start the synthetic probe worker\n")
	fmt.Fprintf(os.Stderr, "  worker   Start recurring database jobs\n")
	fmt.Fprintf(os.Stderr, "  seed     Seed the database with test data\n")
	fmt.Fprintf(os.Stderr, "           --count int   number of services to seed (default 10)\n")
	fmt.Fprintf(os.Stderr, "           --clear       clear existing data before seeding\n")
	fmt.Fprintf(os.Stderr, "  backfill-probe-rollups\n")
	fmt.Fprintf(os.Stderr, "           --start RFC3339   inclusive start time (default 1970-01-01T00:00:00Z)\n")
	fmt.Fprintf(os.Stderr, "           --end RFC3339     exclusive end time (default current UTC hour)\n")
	fmt.Fprintf(os.Stderr, "           --chunk-duration duration   whole-hour chunk size (default 24h, 0 disables)\n")
	fmt.Fprintf(os.Stderr, "  backfill-probe-derived\n")
	fmt.Fprintf(os.Stderr, "           --cutoff RFC3339            required exclusive cutoff time\n")
	fmt.Fprintf(os.Stderr, "           --service-batch-size int    services per batch (default 500)\n")
}

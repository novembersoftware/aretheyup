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
)

type Flags struct {
	Mode Mode

	// mode specific flags
	SeedCount int
	SeedClear bool

	BackfillProbeRollupsStart time.Time
	BackfillProbeRollupsEnd   time.Time
}

var seedFlags = flag.NewFlagSet("seed", flag.ExitOnError)
var backfillProbeRollupsFlags = flag.NewFlagSet("backfill-probe-rollups", flag.ExitOnError)

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
		endDefault := time.Now().UTC().Format(time.RFC3339)
		startRaw := backfillProbeRollupsFlags.String("start", startDefault, "inclusive RFC3339 start time")
		endRaw := backfillProbeRollupsFlags.String("end", endDefault, "exclusive RFC3339 end time")
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
		return Flags{Mode: ModeBackfillProbeRollups, BackfillProbeRollupsStart: start, BackfillProbeRollupsEnd: end}

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
	fmt.Fprintf(os.Stderr, "           --end RFC3339     exclusive end time (default now)\n")
}

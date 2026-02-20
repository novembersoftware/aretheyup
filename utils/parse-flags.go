package utils

import (
	"flag"
	"fmt"
	"os"
)

type Mode string

const (
	ModeAPI    Mode = "api"
	ModeManage Mode = "manage"
	ModeSeed   Mode = "seed"
)

type Flags struct {
	Mode Mode

	// mode specific flags
	SeedCount int
	SeedClear bool
}

var seedFlags = flag.NewFlagSet("seed", flag.ExitOnError)

func ParseFlags() Flags {
	if len(os.Args) < 2 {
		return Flags{Mode: ModeAPI}
	}

	mode := Mode(os.Args[1])

	switch mode {
	case ModeAPI, ModeManage:
		return Flags{Mode: mode}

	case ModeSeed:
		count := seedFlags.Int("count", 10, "number of services to seed")
		clear := seedFlags.Bool("clear", false, "clear existing data before seeding")
		seedFlags.Parse(os.Args[2:])
		return Flags{Mode: ModeSeed, SeedCount: *count, SeedClear: *clear}

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
	fmt.Fprintf(os.Stderr, "  seed     Seed the database with test data\n")
	fmt.Fprintf(os.Stderr, "           --count int   number of services to seed (default 10)\n")
	fmt.Fprintf(os.Stderr, "           --clear       clear existing data before seeding\n")
}

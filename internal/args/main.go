package args

import (
	"flag"
	"fmt"
	"os"
)

type Options struct {
	LockPath               string
	Verbose                bool
	FailIfMultipleVersions bool
	OutputFormat           string
}

func ParseArgs() Options {
	var lockPath string
	var verbose bool
	var failIfMultipleVersions bool
	var outputFormat string

	flag.StringVar(&lockPath, "lockfile", "flake.lock", "path to flake.lock")
	flag.BoolVar(&verbose, "verbose", false, "enable verbose output")
	flag.BoolVar(&failIfMultipleVersions, "fail-if-multiple-versions", false, "exit with error if multiple versions found")
	flag.StringVar(&outputFormat, "output", "plain", "output format: plain or json")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s --lockfile=/path/to/flake.lock --verbose\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --lockfile=/path/to/flake.lock --output=json\n", os.Args[0])
	}

	flag.Parse()

	return Options{
		LockPath:               lockPath,
		Verbose:                verbose,
		FailIfMultipleVersions: failIfMultipleVersions,
		OutputFormat:           outputFormat,
	}
}

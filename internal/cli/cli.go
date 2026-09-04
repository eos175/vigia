package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/eos175/vigia/internal/pidfile"
	"github.com/eos175/vigia/internal/supervisor"
)

const defaultPidfile = ".vigia.pid"

var Version = "dev"

func Run(args []string) int {
	if len(args) == 0 {
		usage(os.Args[0])
		return 1
	}

	switch args[0] {
	case "reload":
		return runReload(args[1:])
	case "version":
		return runVersion()
	default:
		return runSupervisor(args)
	}
}

func runVersion() int {
	fmt.Println(Version)
	return 0
}

func runSupervisor(args []string) int {
	fs := flag.NewFlagSet("vigia", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		alwaysRestart bool
		maxRestarts   int
		pidfilePath   string
		stderrFile    string
	)

	fs.BoolVar(&alwaysRestart, "always-restart", false, "Restart even if process exits cleanly")
	fs.IntVar(&maxRestarts, "max-restarts", 10, "Maximum restart attempts before exiting")
	fs.StringVar(&pidfilePath, "pidfile", defaultPidfile, "Path to pidfile")
	fs.StringVar(&stderrFile, "stderr-file", "", "Path to write duplicated stderr output (optional)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		usage(os.Args[0])
		return 1
	}

	if maxRestarts < 0 {
		fmt.Fprintln(os.Stderr, "--max-restarts must be >= 0")
		return 1
	}

	if pidfilePath == "" {
		fmt.Fprintln(os.Stderr, "--pidfile must not be empty")
		return 1
	}

	return supervisor.Run(supervisor.Config{
		Command:       fs.Arg(0),
		Args:          fs.Args()[1:],
		AlwaysRestart: alwaysRestart,
		MaxRestarts:   maxRestarts,
		PidfilePath:   pidfilePath,
		StderrFile:    stderrFile,
	})
}

func runReload(args []string) int {
	fs := flag.NewFlagSet("vigia reload", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	pidfilePath := fs.String("pidfile", defaultPidfile, "Path to pidfile")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// `reload` stays out-of-process: it only signals the running supervisor.
	if err := pidfile.Reload(*pidfilePath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Println("reloaded")
	return 0
}

func usage(bin string) {
	fmt.Fprintf(os.Stderr, "\nUsage:\n  %s [options] <command> [args...]\n  %s reload [options]\n  %s version\n\n", bin, bin, bin)
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  --always-restart     Restart even if process exits cleanly")
	fmt.Fprintln(os.Stderr, "  --max-restarts int   Maximum restart attempts before exiting (default 10)")
	fmt.Fprintln(os.Stderr, "  --pidfile string     Path to pidfile (default \".vigia.pid\")")
	fmt.Fprintln(os.Stderr, "  --stderr-file string Path to write duplicated stderr output (optional)")
}

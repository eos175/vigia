package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Configurar flags
	alwaysRestart := flag.Bool("always-restart", false, "Restart even if process exits cleanly")
	maxRestarts := flag.Int("max-restarts", 10, "Maximum restart attempts before exiting")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Printf("\nUsage: %s [options] <command> [args...]\n\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	command := flag.Arg(0)
	args := flag.Args()[1:]

	// Configurar zerolog
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	restartCount := 0
	restartDelay := time.Second
	const stabilizationTime = 5 * time.Minute

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		if restartCount >= *maxRestarts {
			log.Error().Int("max_restarts", *maxRestarts).Msg("Maximum restart attempts reached. Exiting.")
			os.Exit(1)
		}

		log.Info().Str("cmd", command).Strs("args", args).Msg("Starting process")

		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, command, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		startTime := time.Now()
		if err := cmd.Start(); err != nil {
			log.Error().Err(err).Msg("Error starting command")
			cancel()
			time.Sleep(restartDelay)
			restartCount++
			continue
		}

		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()

		select {
		case err := <-done:
			cancel()
			if err != nil {
				log.Warn().Err(err).Msg("Process exited with error")
				restartCount++
				log.Info().
					Dur("delay", restartDelay).
					Int("attempt", restartCount).
					Int("max", *maxRestarts).
					Msg("Restarting process")

				time.Sleep(restartDelay)
				restartDelay = time.Duration(min(int(restartDelay.Seconds()*2), 30)) * time.Second
			} else {
				log.Info().Msg("Process completed successfully")
				if *alwaysRestart {
					log.Info().Msg("Always-restart flag is enabled — restarting")
					restartCount = 0
					restartDelay = time.Second
					time.Sleep(1 * time.Second)
					continue
				}
				return
			}

			// Si corrió estable por un tiempo, reinicia el backoff
			if time.Since(startTime) > stabilizationTime {
				restartDelay = time.Second
				restartCount = 0
				log.Info().Msg("Process ran stable — backoff reset")
			}

		case sig := <-sigChan:
			log.Info().Str("signal", sig.String()).Msg("Received termination signal")
			cancel()

			// Esperar apagado limpio
			doneCh := make(chan struct{})
			go func() {
				cmd.Wait()
				close(doneCh)
			}()

			select {
			case <-doneCh:
				log.Info().Msg("Child process exited gracefully")
			case <-time.After(5 * time.Second):
				log.Warn().Msg("Timeout waiting for graceful shutdown — killing process")
				_ = cmd.Process.Kill()
			}
			return
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

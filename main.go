package main

import (
	"context"
	"errors"
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

const (
	stabilizationTime = 5 * time.Minute
	initialBackoff    = 1 * time.Second
	maxBackoff        = 30 * time.Second

	timeoutKill = 10 * time.Second
)

var ErrSignalReceived = errors.New("signal received")

func main() {
	// Configurar flags
	var (
		alwaysRestart bool
		maxRestarts   int
	)

	flag.BoolVar(&alwaysRestart, "always-restart", false, "Restart even if process exits cleanly")
	flag.IntVar(&maxRestarts, "max-restarts", 10, "Maximum restart attempts before exiting")
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
	restartDelay := initialBackoff

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		if restartCount >= maxRestarts {
			log.Error().
				Int("max_restarts", maxRestarts).
				Msg("Maximum restart attempts reached. Exiting.")
			os.Exit(1)
		}

		startTime := time.Now()
		err := run(command, args, sigChan)
		elapsed := time.Since(startTime)

		// 1. Si recibimos una señal de terminación (Ctrl+C), salimos definitivamente
		if err == ErrSignalReceived {
			return
		}

		// 2. Si el proceso terminó exitosamente
		if err == nil {
			log.Info().
				Dur("duration", elapsed).Msg("Process completed successfully")
			if !alwaysRestart {
				return
			}

			log.Info().Msg("Always-restart flag is enabled — restarting")
			restartCount = 0
			restartDelay = initialBackoff
			time.Sleep(1 * time.Second)
			continue
		}

		// 3. Si el proceso falló (err != nil)
		// Resetear backoff si corrió estable por un tiempo
		if time.Since(startTime) > stabilizationTime {
			restartDelay = initialBackoff
			restartCount = 0
			log.Info().Msg("Process ran stable — backoff reset")
		}

		exitCode, _ := exitCodeFromError(err)
		log.Warn().Err(err).
			Dur("duration", elapsed).
			Int("exit_code", exitCode).Msg("Process exited with error")

		restartCount++
		log.Info().
			Dur("delay", restartDelay).
			Int("attempt", restartCount).
			Int("max", maxRestarts).
			Msg("Restarting process")

		time.Sleep(restartDelay)
		restartDelay = nextBackoff(restartDelay, maxBackoff)
	}
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}

func exitCodeFromError(err error) (int, bool) {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return 0, false
	}

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return 0, false
	}

	return status.ExitStatus(), true
}

func run(command string, args []string, sigChan <-chan os.Signal) error {
	log.Info().
		Str("cmd", command).
		Strs("args", args).
		Msg("Starting process")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Garantiza limpieza al terminar la función run()

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Error().Err(err).Msg("Error starting command")
		return err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return err

	case sig := <-sigChan:
		log.Info().
			Str("signal", sig.String()).
			Msg("Received termination signal")

		// Reenviar señal al hijo
		if cmd.Process != nil {
			if err := cmd.Process.Signal(sig); err != nil {
				log.Warn().Err(err).Msg("Failed to forward signal to child process")
			}
		}

		// Esperar apagado limpio o matar si timeout
		select {
		case <-done:
			log.Info().Msg("Child process exited gracefully")
		case <-time.After(timeoutKill):
			log.Warn().Msg("Timeout waiting for graceful shutdown — forcing kill")
			// Al salir de la función, defer cancel() se ejecutará y matará el proceso
		}
		return ErrSignalReceived
	}
}

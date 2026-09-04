package supervisor

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/eos175/vigia/internal/pidfile"
)

const (
	stabilizationTime = 5 * time.Minute
	initialBackoff    = 1 * time.Second
	maxBackoff        = 30 * time.Second
	timeoutKill       = 10 * time.Second
)

var ErrSignalReceived = errors.New("signal received")
var ErrReloadRequested = errors.New("reload requested")

type Config struct {
	Command       string
	Args          []string
	AlwaysRestart bool
	MaxRestarts   int
	PidfilePath   string
	StderrFile    string
}

func Run(cfg Config) int {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	var stderrWriter io.Writer = os.Stderr
	if cfg.StderrFile != "" {
		file, err := os.OpenFile(cfg.StderrFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Error().Err(err).Str("file", cfg.StderrFile).Msg("Failed to open stderr log file")
			return 1
		}
		defer file.Close()
		stderrWriter = io.MultiWriter(os.Stderr, file)
	}

	// Expose the supervisor PID so `vigia reload` can target this exact instance.
	// Check if an existing supervisor is already running to avoid overwriting the pidfile.
	if pid, err := pidfile.Read(cfg.PidfilePath); err == nil {
		if err := syscall.Kill(pid, 0); err == nil {
			log.Error().Int("pid", pid).Str("pidfile", cfg.PidfilePath).Msg("Supervisor already running")
			return 1
		}
	}

	if err := pidfile.Write(cfg.PidfilePath, os.Getpid()); err != nil {
		log.Error().Err(err).Str("pidfile", cfg.PidfilePath).Msg("Failed to write pidfile")
		return 1
	}
	defer func() { _ = pidfile.Remove(cfg.PidfilePath) }()

	restartCount := 0
	restartDelay := initialBackoff

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	defer signal.Stop(sigChan)

	for {
		if restartCount >= cfg.MaxRestarts {
			log.Error().Int("max_restarts", cfg.MaxRestarts).Msg("Maximum restart attempts reached. Exiting.")
			return 1
		}

		startTime := time.Now()
		err := run(cfg.Command, cfg.Args, stderrWriter, sigChan)
		elapsed := time.Since(startTime)

		if err == ErrReloadRequested {
			log.Info().Msg("Reload requested")
			restartCount = 0
			restartDelay = initialBackoff
			continue
		}

		if err == ErrSignalReceived {
			return 0
		}

		if err == nil && !cfg.AlwaysRestart {
			log.Info().Dur("duration", elapsed).Msg("Process completed successfully")
			return 0
		}

		if err == nil {
			log.Info().Dur("duration", elapsed).Msg("Process completed successfully")
			log.Info().Msg("Always-restart flag is enabled — restarting")
		} else {
			exitCode, _ := exitCodeFromError(err)
			log.Warn().Err(err).Dur("duration", elapsed).Int("exit_code", exitCode).Msg("Process exited with error")
		}

		if time.Since(startTime) > stabilizationTime {
			restartCount = 0
			restartDelay = initialBackoff
			log.Info().Msg("Process ran stable — backoff reset")
		}

		restartCount++
		log.Info().Dur("delay", restartDelay).Int("attempt", restartCount).Int("max", cfg.MaxRestarts).Msg("Restarting process")

		if sig, ok := waitForSignalOrTimeout(restartDelay, sigChan); ok {
			if sig != syscall.SIGUSR1 {
				return 0
			}
			log.Info().Msg("Reload requested")
			restartCount = 0
			restartDelay = initialBackoff
			continue
		}
		restartDelay = nextBackoff(restartDelay, maxBackoff)
	}
}

func run(command string, args []string, stderrWriter io.Writer, sigChan <-chan os.Signal) error {
	log.Info().Str("cmd", command).Strs("args", args).Msg("Starting process")

	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = stderrWriter
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
		if sig == syscall.SIGUSR1 {
			// Reload means: stop the child, then loop back and start it again.
			if err := signalProcessGroup(cmd.Process.Pid, syscall.SIGTERM); err != nil {
				log.Warn().Err(err).Msg("Failed to forward reload termination to child process group")
			}
			select {
			case <-done:
			case <-time.After(timeoutKill):
				log.Warn().Msg("Timeout waiting for graceful shutdown during reload — forcing kill")
				if err := signalProcessGroup(cmd.Process.Pid, syscall.SIGKILL); err != nil {
					log.Warn().Err(err).Msg("Failed to force kill child process group")
				}
				<-done
			}
			return ErrReloadRequested
		}

		log.Info().Str("signal", sig.String()).Msg("Received termination signal")
		if err := signalProcessGroup(cmd.Process.Pid, sig); err != nil {
			log.Warn().Err(err).Msg("Failed to forward signal to child process group")
		}
		select {
		case <-done:
			log.Info().Msg("Child process exited gracefully")
		case <-time.After(timeoutKill):
			log.Warn().Msg("Timeout waiting for graceful shutdown — forcing kill")
			if err := signalProcessGroup(cmd.Process.Pid, syscall.SIGKILL); err != nil {
				log.Warn().Err(err).Msg("Failed to force kill child process group")
			}
			<-done
		}
		return ErrSignalReceived
	}
}

func waitForSignalOrTimeout(delay time.Duration, sigChan <-chan os.Signal) (os.Signal, bool) {
	if delay <= 0 {
		return nil, false
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil, false
	case sig := <-sigChan:
		return sig, true
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
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		return 0, false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return 0, false
	}
	return status.ExitStatus(), true
}

func signalProcessGroup(pid int, sig os.Signal) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}
	sysSig, ok := sig.(syscall.Signal)
	if !ok {
		return fmt.Errorf("unsupported signal type %T", sig)
	}
	if err := syscall.Kill(-pid, sysSig); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	return nil
}

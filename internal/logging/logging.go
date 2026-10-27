package logging

import (
	"os"
	"sync"

	"github.com/rs/zerolog"
)

var (
	logger zerolog.Logger
	once   sync.Once
)

func Init(debug, verbose bool) {
	once.Do(func() {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

		if debug {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		} else if verbose {
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		} else {
			zerolog.SetGlobalLevel(zerolog.WarnLevel)
		}

		// stderr: stdout carries the SQL, table or JSON, which a log line would corrupt.
		logger = zerolog.New(os.Stderr).With().Timestamp().Caller().Logger()
	})
}

func Debug(msg string) {
	logger.Debug().Msg(msg)
}

func Info(msg string) {
	logger.Info().Msg(msg)
}

func Warn(msg string) {
	logger.Warn().Msg(msg)
}

func Error(msg string) {
	logger.Error().Msg(msg)
}

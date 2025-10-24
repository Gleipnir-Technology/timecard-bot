package main

import (
	"log"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/util/exzerolog"
)

type Logger = zerolog.Logger

func CreateLogger() *Logger {
	l := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		//w.Out = io.Writer(buf)
		w.TimeFormat = time.Stamp
	})).With().Timestamp().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	exzerolog.SetupDefaults(&l)
	return &l
}

type ZerologWriter struct {
	zerologger zerolog.Logger
	level      zerolog.Level
}

func (w ZerologWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSuffix(string(p), "\n")
	event := w.zerologger.WithLevel(w.level)
	event.Msg(msg)
	return len(p), nil
}

func LoggerShim(l zerolog.Logger) *log.Logger {
	writer := &ZerologWriter{
		zerologger: l,
		level:      zerolog.DebugLevel,
	}
	return log.New(writer, "", 0)
}

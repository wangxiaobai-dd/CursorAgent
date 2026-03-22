package util

import (
	"io"
	"log"
	"os"
)

var noopLogClose = func() {}

func SetupLogOutput(logFile string) (close func(), err error) {
	f, err := OpenAppendFile(logFile)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return noopLogClose, nil
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	return func() { _ = f.Close() }, nil
}

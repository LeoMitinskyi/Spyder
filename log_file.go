package main

import (
	"os"
	"sync"
	"time"
)

type LogFile struct {
	mu   sync.Mutex
	name string
	file *os.File
}

func NewLogFile(name string, file *os.File) (*LogFile, error) {
	rw := &LogFile{file: file, name: name}
	if file == nil {
		if err := rw.Rotate(); err != nil {
			return nil, err
		}
	}
	return rw, nil
}

func (l *LogFile) Write(b []byte) (n int, err error) {
	l.mu.Lock()
	n, err = l.file.Write(b)
	l.mu.Unlock()
	return
}

func (l *LogFile) Rotate() error {
	if _, err := os.Stat(l.name); err == nil {
		name := l.name + "." + time.Now().Format(time.RFC3339)
		if err = os.Rename(l.name, name); err != nil {
			return err
		}
	}

	file, err := os.Create(l.name)
	if err != nil {
		return err
	}

	l.mu.Lock()
	file, l.file = l.file, file
	l.mu.Unlock()

	if file != nil {
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

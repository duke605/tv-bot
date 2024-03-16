package utils

import (
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/spf13/afero"
)

type dateFile struct {
	fs       afero.Fs
	file     afero.File
	lastDate string
	name     string
	flags    int
	perms    fs.FileMode
}

func NewDateFile(fs afero.Fs, name string, flags int, perms fs.FileMode) io.ReadWriteCloser {
	return &dateFile{
		fs:    fs,
		name:  name,
		flags: flags,
		perms: perms,
	}
}

func (f *dateFile) Close() error {
	if f.file != nil {
		return nil
	}

	return f.Close()
}

func (f *dateFile) loadFile() error {
	t := time.Now().Format(time.DateOnly)
	if t == f.lastDate {
		return nil
	}

	if f.file != nil {
		if err := f.file.Close(); err != nil {
			return err
		}
	}

	name := fmt.Sprintf("%s-%s", t, f.name)
	file, err := f.fs.OpenFile(name, f.flags, f.perms)
	if err != nil {
		return err
	}

	f.file = file
	return nil
}

func (f *dateFile) Write(b []byte) (int, error) {
	if err := f.loadFile(); err != nil {
		return 0, err
	}

	return f.file.Write(b)
}

func (f *dateFile) Read(b []byte) (n int, err error) {
	if err := f.loadFile(); err != nil {
		return 0, err
	}

	return f.file.Read(b)
}

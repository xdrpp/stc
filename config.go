
package main

import (
	"fmt"
	"io"
	"os"
)

var ConfigDir string

func init() {
	if d, ok := os.LookupEnv("STCDIR"); ok {
		ConfigDir = d
	} else if d, ok = os.LookupEnv("XDG_CONFIG_HOME"); ok {
		ConfigDir = d + "/stc"
	} else if d, ok = os.LookupEnv("HOME"); ok {
		ConfigDir = d + "/.config/stc"
	} else {
		ConfigDir = ".stc"
	}
}

func SafeWriteFile(filename string, data []byte, perm os.FileMode) error {
	tmp := fmt.Sprintf("%s#%d#", filename, os.Getpid())
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		if f != nil { f.Close() }
		if tmp != "" { os.Remove(tmp) }
	}()

	n, err := f.Write(data)
	if err != nil {
		return err
	} else if n < len(data) {
		return io.ErrShortWrite
	}
	if err = f.Sync(); err != nil {
		return err
	}
	err = f.Close()
	f = nil
	if err != nil {
		return err
	}

	os.Remove(filename + "~")
	os.Link(filename, filename + "~")
	err = os.Rename(tmp, filename)
	tmp = ""
	return err
}

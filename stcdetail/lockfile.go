package stcdetail

import (
	"os"
)

type ErrIsDirectory string
func (e ErrIsDirectory) Error() string {
	return string(e) + ": is a directory"
}

// Updates a file in a safe way, by first writing data to an
// exclusively lockfile (which is path with ".lock" appended).  If the
// action function returns a nil error, then the file is flushed and
// renamed to path.  This is the same scheme git uses to lock
// configuration files.
func UpdateFile(path string, perm os.FileMode,
	action func(*os.File)error) (err error) {
	if path == "" {
		return os.ErrInvalid
	} else if fi, e := os.Stat(path); e != nil && !os.IsNotExist(e) {
		return e
	} else if e == nil && fi.Mode().IsDir() {
		return ErrIsDirectory(path)
	}

	lockpath := path + ".lock"
	var f *os.File
	f, err = os.OpenFile(lockpath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return
	}
	defer func() {
		if f != nil {
			f.Close()
		}
		if lockpath != "" {
			os.Remove(lockpath)
		}
	}()
	if err = action(f); err != nil {
		return
	} else if err = f.Sync(); err != nil {
		return
	}
	if err, f = f.Close(), nil; err != nil {
		return
	}

	tildepath := path + "~"
	os.Remove(tildepath)
	os.Link(path, tildepath)
	if err = os.Rename(lockpath, path); err == nil {
		lockpath = ""
	}
	return
}

// Writes data tile filename in a safe way.  If path is "foo", then
// data is first written to a file called "foo.lock" and that file is
// flushed to disk.  Then, if a file called "foo" already exists,
// "foo" is linked to "foo~" to keep a backup.  Finally, "foo.lock" is
// renamed to "foo".  Fails if "foo.lock" already exists.
func SafeWriteFile(path string, data string, perm os.FileMode) error {
	return UpdateFile(path, perm, func(f *os.File) error {
		n, err := f.WriteString(data)
		if err != nil {
			return err
		} else if n != len(data) {
			panic("Short write should have returned an error")
		}
		return nil
	})
}

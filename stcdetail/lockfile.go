package stcdetail

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
)

type ErrIsDirectory string
func (e ErrIsDirectory) Error() string {
	return string(e) + ": is a directory"
}

type ErrFileHasChanged string
func (e ErrFileHasChanged) Error() string {
	return string(e) + ": file has changed since read"
}

var ErrAborted = fmt.Errorf("Aborted")

func clearAtime(sys interface{}) bool {
	v := reflect.ValueOf(sys)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}
	atv := v.FieldByNameFunc(func(s string) bool {
		return strings.HasPrefix(s, "Atim")
	})
	if (atv != reflect.Value{}) {
		atv.Set(reflect.Zero(atv.Type()))
		return true
	}
	return false
}

// Return true if a file may have been changed between the times that
// the two FileInfo structures were read.
func FileChanged(a, b os.FileInfo) bool {
	if a.ModTime() != b.ModTime() || a.Size() != b.Size() {
		return true
	}
	sa, sb := a.Sys(), b.Sys()
	if !clearAtime(sa) || !clearAtime(sb) {
		fmt.Fprintf(os.Stderr, "Can't parse FileInfo.Sys()\n")
	}
	return !reflect.DeepEqual(a, b)
}

func ReadFile(path string) ([]byte, os.FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	ret, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}
	return ret, fi, nil
}

type lockedFile struct {
	path string
	lockpath string
	f *os.File
	err error
	fi os.FileInfo
}

func (lf *lockedFile) Abort() {
	if lf.f != nil {
		lf.f.Close()
		lf.f = nil
	}
	if lf.lockpath != "" {
		os.Remove(lf.lockpath)
		lf.lockpath = ""
	}
	if lf.err == nil {
		lf.err = ErrAborted
	}
}

func (lf *lockedFile) check(err error) error {
	if lf.err == nil && err != nil {
		lf.err = err
		lf.Abort()
	}
	return lf.err
}

func (lf *lockedFile) Status() error {
	if lf.err != nil {
		return lf.err
	}
	fi, err := os.Stat(lf.path)
	if !os.IsNotExist(err) && lf.check(err) != nil {
		return lf.err
	}
	if lf.fi == nil && err == nil ||
		lf.fi != nil && fi != nil && FileChanged(fi, lf.fi) {
		return lf.check(ErrFileHasChanged(lf.path))
	}
	return nil
}

func (lf *lockedFile) Write(b []byte) (int, error) {
	if lf.err != nil {
		return 0, lf.err
	}
	n, err := lf.f.Write(b)
	return n, lf.check(err)
}

func (lf *lockedFile) WriteString(s string) (int, error) {
	if lf.err != nil {
		return 0, lf.err
	}
	n, err := lf.f.WriteString(s)
	return n, lf.check(err)
}

func (lf *lockedFile) Commit() error {
	if lf.f == nil {
		return lf.err
	}
	if lf.check(lf.f.Sync()) != nil {
		return lf.err
	}
	err := lf.f.Close()
	lf.f = nil
	if lf.check(err) != nil {
		return lf.err
	}

	tildepath := lf.path + "~"
	os.Remove(tildepath)
	os.Link(lf.path, tildepath)
	ret := lf.check(os.Rename(lf.lockpath, lf.path))
	lf.lockpath = ""
	if lf.err == nil {
		lf.err = os.ErrInvalid
	}
	return ret
}

func (lf *lockedFile) ReadFile() ([]byte, error) {
	if lf.err != nil {
		return nil, lf.err
	}
	if ret, fi, err := ReadFile(lf.path); err == nil {
		if lf.fi != nil && FileChanged(lf.fi, fi) {
			return nil, lf.check(ErrFileHasChanged(lf.path))
		}
		lf.fi = fi
		return ret, nil
	} else if !os.IsNotExist(err) {
		return nil, lf.check(err)
	} else {
		return ret, err
	}
}

// An interface to update a file atomically.  Acts as a Writer, and
// when you call Commit() atomically renames the lockfile you just
// wrote to the target filename.
type LockedFile interface {
	io.Writer
	io.StringWriter

	// Call when you wish to replace the locked file with what you
	// have written to this Writer and then release the lock.
	Commit() error

	// Returns the contents of the locked file.  (This is not the
	// contents of the lockfile itself, which is initially empty and
	// to which you must write the new contents you want to swap in
	// atomically.)  Also checks the file modification time, so as to
	// cause an abort if anyone else changes the file after you've
	// locked it and before you've called Commit().
	ReadFile() ([]byte, error)

	// You must call Abort() to clean up the lockfile, unless you have
	// called Commit().  However, it is safe to call Abort() multiple
	// times, or to call Abort() after Commit(), so the best use is to
	// call defer lf.Abort() as soon as you have a lockedfile.
	Abort()

	// A LockedFile accumulates errors from the Write() function so
	// you don't have to check every time you write.  If there has
	// been an error, Status() will return it.
	Status() error
}

// Locks a file for updating.  Exclusively creates a file with name
// path + ".lock", returns a writer that lets you write into this
// lockfile, and then when you call Commit() replaces path with what
// you have just written.  You must call Abort() or Commit() on the
// returned interface.  Since it is safe to call both, best practice
// is to defer a call to Abort() immediately.
func LockFile(path string, perm os.FileMode) (LockedFile, error) {
	lf := lockedFile{
		path: path,
		lockpath: path + ".lock",
	}
	if path == "" {
		return nil, os.ErrInvalid
	} else if fi, e := os.Stat(path); e != nil && !os.IsNotExist(e) {
		return nil, e
	} else if e == nil && fi.Mode().IsDir() {
		return nil, ErrIsDirectory(path)
	} else if e == nil {
		// Would be impolite to increase permissions...
		perm &= fi.Mode()
		lf.fi = fi
	}

	f, err := os.OpenFile(lf.lockpath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return nil, err
	}
	lf.f = f
	return &lf, nil
}

// Writes data tile filename in a safe way.  If path is "foo", then
// data is first written to a file called "foo.lock" and that file is
// flushed to disk.  Then, if a file called "foo" already exists,
// "foo" is linked to "foo~" to keep a backup.  Finally, "foo.lock" is
// renamed to "foo".  Fails if "foo.lock" already exists.
func SafeWriteFile(path string, data string, perm os.FileMode) error {
	lf, err := LockFile(path, perm)
	if err != nil {
		return err
	}
	defer lf.Abort()
	lf.WriteString(data)
	return lf.Commit()
}

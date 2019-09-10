package stcdetail

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
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
// the two FileInfo structures were read.  Note that it modifies the
// two arguments to set the atimes to zero before doing a deep compare
// of all fields.  (This ensures files will be considered changed if
// their ctimes differ, even if the mtimes are the same.)
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

// Read the contents of a file and also return the FileInfo at the
// time the file is read.  The returned FileInfo can be checked with
// FileChanged to see if the file has changed since it was last read,
// and the result will be guaranteed not to miss modifications (at
// least on Unix where setting the mtime to something in the past
// increases the ctime).
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
	path     string
	lockpath string
	f        *os.File
	*bufio.Writer
	fi os.FileInfo
}

func (lf *lockedFile) Abort() {
	if lf.f != nil {
		lf.f.Close()
		lf.f = nil
	}
	lf.Writer = nil
	if lf.lockpath != "" {
		os.Remove(lf.lockpath)
		lf.lockpath = ""
	}
}

type errAccum struct {
	error
}

func (ea *errAccum) accum(err error) error {
	if ea.error == nil && err != nil {
		ea.error = err
	}
	return ea.error
}

func (lf *lockedFile) Status() os.FileInfo {
	return lf.fi
}

func (lf *lockedFile) Commit() error {
	var ea errAccum
	ea.accum(lf.Flush())
	lf.Writer = nil
	ea.accum(lf.f.Sync())
	fi0, err := lf.f.Stat()
	ea.accum(err)
	ea.accum(lf.f.Close())
	lf.f = nil
	if ea.error != nil {
		lf.Abort()
		return ea.error
	}

	if fi, err := os.Stat(lf.path); err == nil &&
		(lf.fi == nil || FileChanged(lf.fi, fi)) {
		lf.Abort()
		return ErrFileHasChanged(lf.path)
	} else if fi, err = os.Stat(lf.lockpath); err != nil {
		lf.Abort()
		return err
	} else if FileChanged(fi0, fi) {
		err = ErrFileHasChanged(lf.lockpath)
		lf.Abort()
		return err
	}

	tildepath := lf.path + "~"
	os.Remove(tildepath)
	os.Link(lf.path, tildepath)

	if ea.accum(os.Rename(lf.lockpath, lf.path)) == nil {
		lf.lockpath = ""
	}
	lf.Abort()
	return ea.error
}

func (lf *lockedFile) ReadFile() ([]byte, error) {
	if ret, fi, err := ReadFile(lf.path); err != nil {
		return ret, err
	} else if lf.fi == nil || FileChanged(lf.fi, fi) {
		return nil, ErrFileHasChanged(lf.path)
	} else {
		lf.fi = fi
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
	// to which you must write the new contents that you want to swap
	// in atomically.)  Also checks the file modification time, so as
	// to cause an abort if anyone else changes the file after you've
	// locked it and before you've called Commit().
	ReadFile() ([]byte, error)

	// Return the file info at the time the lock was taken (or nil if
	// the file did not exist).
	Status() os.FileInfo

	// You must call Abort() to clean up the lockfile, unless you have
	// called Commit().  However, it is safe to call Abort() multiple
	// times, or to call Abort() after Commit(), so the best use is to
	// call defer lf.Abort() as soon as you have a LockedFile.
	Abort()
}

func doLockFile(path string, perm os.FileMode,
	readfi os.FileInfo) (LockedFile, error) {
	if phys, err := filepath.EvalSymlinks(path); err == nil {
		path = phys
	}
	lf := lockedFile{
		path:     path,
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

	if readfi != nil && (lf.fi == nil || FileChanged(readfi, lf.fi)) {
		return nil, ErrFileHasChanged(path)
	}

	f, err := os.OpenFile(lf.lockpath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return nil, err
	}
	lf.f = f
	lf.Writer = bufio.NewWriter(lf.f)
	return &lf, nil
}


// Locks a file for updating.  Exclusively creates a file with name
// path + ".lock", returns a writer that lets you write into this
// lockfile, and then when you call Commit() replaces path with what
// you have just written.  You must call Abort() or Commit() on the
// returned interface.  Since it is safe to call both, best practice
// is to defer a call to Abort() immediately.
func LockFile(path string, perm os.FileMode) (LockedFile, error) {
	return doLockFile(path, perm, nil)
}

// Like LockFile, but fails if file's stat information (other than
// atime) does not exactly match fi.  If fi is nil, then acts like
// LockFile with perm of 0666.
func LockFileIfUnchanged(path string, fi os.FileInfo) (LockedFile, error) {
	if fi != nil {
		return doLockFile(path, fi.Mode() & os.ModePerm, fi)
	} else {
		return doLockFile(path, 0666, nil)
	}
}

// Writes data to file path in a safe way.  If path is "foo", then
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

// Like SafeWriteFile, but fails if the file already exists after the
// lock is acquired.  Does not exclusively create the target file, but
// rather uses a lockfile to ensure that if the file is created it
// will have data as its contents.  (Exclusively creating the target
// file could lead to the file existing but being empty after a
// crash.)
func SafeCreateFile(path string, data string, perm os.FileMode) error {
	lf, err := LockFile(path, perm)
	if err != nil {
		return err
	}
	defer lf.Abort()
	if lf.(*lockedFile).fi != nil {
		return os.ErrExist
	}
	lf.WriteString(data)
	return lf.Commit()
}

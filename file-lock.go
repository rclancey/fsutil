package fsutil

import (
	"io"
	"log"
	"os"
	"syscall"

	"github.com/pkg/errors"
)

type LockedFile struct {
	*os.File
	flag int
	lock *int
}

func OpenLocked(name string, flag int, perm os.FileMode) (*LockedFile, error) {
	trunc := (flag & os.O_TRUNC) == 0 && (flag & (os.O_WRONLY | os.O_RDWR)) == 0
	flag = flag & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR | os.O_APPEND | os.O_CREATE | os.O_EXCL | os.O_SYNC)
	f, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	lf := &LockedFile{f, flag & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR), nil}
	err = lf.Lock()
	if err != nil {
		f.Close()
		return nil, err
	}
	if trunc {
		err = f.Truncate(0)
		if err != nil {
			f.Close()
			return nil, err
		}
	}
	return lf, nil
}

func (lf *LockedFile) Lock() error {
	if lf.lock != nil {
		// already locked
		return nil
	}
	var lock int
	if lf.flag & os.O_RDONLY != 0 {
		lock = syscall.LOCK_SH
	} else {
		lock = syscall.LOCK_EX
	}
	log.Println("lock", lf.File.Name())
	err := syscall.Flock(int(lf.Fd()), lock)
	if err != nil {
		return err
	}
	log.Println("got lock for", lf.File.Name())
	lf.lock = &lock
	return nil
}

func (lf *LockedFile) Unlock() error {
	if lf.lock == nil {
		return nil
	}
	log.Println("unlocking", lf.File.Name())
	err := syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)
	if err != nil {
		return err
	}
	log.Println("unlocked", lf.File.Name())
	lf.lock = nil
	return nil
}

func (lf *LockedFile) Close() error {
	err := lf.Sync()
	if err != nil {
		log.Println("error syncing file before closing:", err)
		return err
	}
	err = lf.Unlock()
	if err != nil {
		return err
	}
	return lf.File.Close()
}

func ReadLocked(fn string, callback func(io.ReadSeeker) error) error {
	f, err := os.Open(fn)
	if err != nil {
		return errors.Wrapf(err, "error opening file %s", fn)
	}
	defer f.Close()
	fd := int(f.Fd())
	err = syscall.Flock(fd, syscall.LOCK_SH)
	if err != nil {
		return errors.Wrapf(err, "error getting read lock on %s", fn)
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	return callback(f)
}

func CreateLocked(fn string, callback func(io.Writer) error) error {
	f, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0664)
	if err != nil {
		return errors.Wrapf(err, "error creating file %s", fn)
	}
	defer f.Close()
	fd := int(f.Fd())
	err = syscall.Flock(fd, syscall.LOCK_EX)
	if err != nil {
		return errors.Wrapf(err, "error getting write lock on %s", fn)
	}
	err = callback(f)
	if err != nil {
		syscall.Flock(fd, syscall.LOCK_UN)
		f.Close()
		return err
	}
	err = f.Sync()
	if err != nil {
		syscall.Flock(fd, syscall.LOCK_UN)
		f.Close()
		return err
	}
	err = syscall.Flock(fd, syscall.LOCK_UN)
	if err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func UpdateLocked(fn string, callback func(io.ReadSeeker, io.Writer) error) error {
	rf, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0664)
	if err != nil {
		return errors.Wrapf(err, "error opening file %s", fn)
	}
	defer rf.Close()
	fd := int(rf.Fd())
	err = syscall.Flock(fd, syscall.LOCK_EX)
	if err != nil {
		return errors.Wrapf(err, "error getting write lock on %s", fn)
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	wf, err := os.OpenFile(fn + ".tmp", os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return errors.Wrapf(err, "error creating tempfile %s.tmp", fn)
	}
	defer os.Remove(fn + ".tmp")
	defer wf.Close()
	err = callback(rf, wf)
	if err != nil {
		return err
	}
	err = wf.Sync()
	if err != nil {
		return errors.Wrapf(err, "error flushing tempfile %s.tmp", fn)
	}
	wf.Seek(0, os.SEEK_SET)
	rf.Seek(0, os.SEEK_SET)
	rf.Truncate(0)
	_, err = io.Copy(rf, wf)
	if err != nil {
		return errors.Wrapf(err, "error writing changes to %s", fn)
	}
	err = rf.Sync()
	if err != nil {
		return errors.Wrapf(err, "error flushing changes to %s", fn)
	}
	return nil
}

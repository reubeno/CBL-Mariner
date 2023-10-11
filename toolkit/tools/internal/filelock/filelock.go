package filelock

import (
	"os"
	"syscall"
)

type FileLock struct {
	file *os.File
}

func NewLock(path string) (*FileLock, error) {
	// Try creating the lock file (or opening if it exists).
	createdFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	lock := &FileLock{
		file: createdFile,
	}

	return lock, nil
}

func (l *FileLock) Close() error {
	if l.file != nil {
		return l.file.Close()
	} else {
		return nil
	}
}

func (l *FileLock) LockExclusive() error {
	return syscall.Flock(int(l.file.Fd()), syscall.LOCK_EX)
}

func (l *FileLock) LockShared() error {
	return syscall.Flock(int(l.file.Fd()), syscall.LOCK_SH)
}

func (l *FileLock) TryLockExclusive() (bool, error) {
	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == syscall.EWOULDBLOCK {
		return false, nil
	} else if err != nil {
		return false, err
	} else {
		return true, nil
	}
}

func (l *FileLock) TryLockShared() (bool, error) {
	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
	if err == syscall.EWOULDBLOCK {
		return false, nil
	} else if err != nil {
		return false, err
	} else {
		return true, nil
	}
}

func (l *FileLock) Unlock() error {
	return syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
}

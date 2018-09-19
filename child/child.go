package child

import (
	"errors"
	"os"
	"syscall"
)

var (
	// ErrZeroMasterPID returned when given master PID is zero
	ErrZeroMasterPID = errors.New("master PID is zero")
)

// NotifyMaster notifies the master about child readyness
func NotifyMaster(masterPID int) error {
	if masterPID == 0 {
		return ErrZeroMasterPID
	}

	proc, err := os.FindProcess(masterPID)
	if err != nil {
		return err
	}

	return proc.Signal(syscall.SIGUSR1)
}

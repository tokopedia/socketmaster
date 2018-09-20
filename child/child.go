package child

import (
	"errors"
	"os"
	"strconv"
	"syscall"
)

var (
	// ErrZeroMasterPID returned when given master PID is zero
	ErrZeroMasterPID = errors.New("master PID is zero or empty")
)

// NotifyMaster notifies the master about child readyness
func NotifyMaster() error {
	pidStr := os.Getenv("SOCKETMASTER_PID")
	if pidStr == "" {
		return ErrZeroMasterPID
	}

	masterPID, err := strconv.Atoi(pidStr)
	if err != nil {
		return err
	}
	if masterPID == 0 {
		return ErrZeroMasterPID
	}

	proc, err := os.FindProcess(masterPID)
	if err != nil {
		return err
	}

	return proc.Signal(syscall.SIGUSR1)
}

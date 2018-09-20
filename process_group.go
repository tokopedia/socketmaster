package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"strconv"
	"sync"
	"syscall"
)

type ProcessGroup struct {
	set *processSet
	wg  sync.WaitGroup

	commandPath string
	sockfile    *os.File
	user        *user.User

	// if true, socketmaster will wait SIGUSR1 from the new child
	// before killing the old one
	waitChildNotif bool
}

type processSet struct {
	sync.Mutex
	set map[*os.Process]bool
}

func MakeProcessGroup(commandPath string, sockfile *os.File, u *user.User, waitChildNotif bool) *ProcessGroup {
	return &ProcessGroup{
		set:            newProcessSet(),
		commandPath:    commandPath,
		sockfile:       sockfile,
		user:           u,
		waitChildNotif: waitChildNotif,
	}
}

func (self *ProcessGroup) StartProcess() (process *os.Process, err error) {
	self.wg.Add(1)

	ioReader, ioWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	env := append(os.Environ(), "EINHORN_FDS=3")
	if self.waitChildNotif {
		env = append(env, fmt.Sprintf("SOCKETMASTER_PID=%d", os.Getpid()))
	}

	procAttr := &os.ProcAttr{
		Env:   env,
		Files: []*os.File{os.Stdin, ioWriter, ioWriter, self.sockfile},
		Sys:   &syscall.SysProcAttr{},
	}

	if self.user != nil {
		uid, _ := strconv.Atoi(self.user.Uid)
		gid, _ := strconv.Atoi(self.user.Gid)

		procAttr.Sys.Credential = &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		}
	}

	args := append([]string{self.commandPath}, flag.Args()...)

	log.Println("Starting", self.commandPath, args)
	process, err = os.StartProcess(self.commandPath, args, procAttr)
	if err != nil {
		return
	}

	// Add to set
	self.set.Add(process)

	// Prefix stdout and stderr lines with the [pid] and send it to the log
	go logOutput(ioReader, process.Pid, self.wg)

	// Handle the process death
	go func() {
		state, err := process.Wait()

		log.Println(process.Pid, state, err)

		// Remove from set
		self.set.Remove(process)

		// Process is gone
		ioReader.Close()
		self.wg.Done()
	}()

	return
}

func (self *ProcessGroup) SignalAll(signal os.Signal, except *os.Process) {
	self.set.Each(func(process *os.Process) {
		if process != except {
			process.Signal(signal)
		}
	})
}

func (self *ProcessGroup) WaitAll() {
	self.wg.Wait()
}

// A thread-safe process set
func newProcessSet() *processSet {
	set := new(processSet)
	set.set = make(map[*os.Process]bool)
	return set
}

func (self *processSet) Len() int {
	self.Lock()
	defer self.Unlock()

	return len(self.set)
}

func (self *processSet) Add(process *os.Process) {
	self.Lock()
	defer self.Unlock()

	self.set[process] = true
}

func (self *processSet) Each(fn func(*os.Process)) {
	self.Lock()
	defer self.Unlock()

	for process, _ := range self.set {
		fn(process)
	}
}

func (self *processSet) Remove(process *os.Process) {
	self.Lock()
	defer self.Unlock()
	delete(self.set, process)
}

func logOutput(input *os.File, pid int, wg sync.WaitGroup) {
	var err error
	var line string
	wg.Add(1)

	reader := bufio.NewReader(input)

	for err == nil {
		line, err = reader.ReadString('\n')
		if line != "" {
			log.Printf("[%d] %s", pid, line)
		}
	}

	wg.Done()
}

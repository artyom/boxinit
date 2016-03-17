// +build linux

package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
)

func main() {
	var mountProc bool
	flag.BoolVar(&mountProc, "withproc", mountProc, "mount /proc inside container (docker doesn't need this)")
	flag.Parse()
	if len(flag.Args()) == 0 {
		log.Fatal("no command to run")
	}
	if mountProc {
		if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
			log.Fatal("failed to mount /proc: " + err.Error())
		}
	}
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
	)
	ps := newProcSet()
	go func() {
		for sig := range sigch {
			log.Printf("%s, propagating signal to children", sig)
			ps.Signal(sig)
		}
	}()
	for _, arg := range flag.Args() {
		cmd := exec.Command(arg)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			ps.Signal(syscall.SIGCONT)
			ps.Signal(syscall.SIGTERM)
			log.Fatal(err)
		}
		ps.Add(cmd.Process)
	}
	finishing := false
	var ws syscall.WaitStatus
	var mainExitCode int
	for {
		pid, err := syscall.Wait4(-1, &ws, 0, nil)
		if err != nil {
			log.Print(err)
			os.Exit(mainExitCode)
		}
		if ps.HasPid(pid) {
			if finishing {
				log.Printf("watched pid %d finished (%s)",
					pid, exitReason(ws))
				continue
			}
			log.Printf("watched pid %d finished (%s), terminating others",
				pid, exitReason(ws))
			mainExitCode = ws.ExitStatus()
			ps.Signal(syscall.SIGCONT)
			ps.Signal(syscall.SIGTERM)
			finishing = true
		}
	}
}

func init() {
	log.SetFlags(0)
	log.SetPrefix("boxinit: ")
}

func exitReason(ws syscall.WaitStatus) string {
	if ws.Signaled() {
		return ws.Signal().String()
	}
	return "code " + strconv.Itoa(ws.ExitStatus())
}

func newProcSet() *procSet {
	return &procSet{
		h: make(map[*os.Process]struct{}),
	}
}

type procSet struct {
	m sync.RWMutex
	h map[*os.Process]struct{}
}

func (ps *procSet) HasPid(pid int) bool {
	ps.m.RLock()
	defer ps.m.RUnlock()
	for p := range ps.h {
		if p.Pid == pid {
			return true
		}
	}
	return false
}

func (ps *procSet) Add(p *os.Process) {
	ps.m.Lock()
	ps.h[p] = struct{}{}
	ps.m.Unlock()
}

func (ps *procSet) Signal(sig os.Signal) {
	// if we're really working as init (pid 1), send signal to all
	// processes except ourself
	if syscall.Getpid() == 1 {
		if err := syscall.Kill(-1, sig.(syscall.Signal)); err != nil {
			log.Print("error sending signal: ", err)
		}
		return
	}
	// send signal only to processes we explicitly spawned
	ps.m.RLock()
	for p := range ps.h {
		p.Signal(sig)
	}
	ps.m.RUnlock()
}

//go:build linux

package roomhost

import (
	"os/exec"
	"syscall"
)

func configureWorkerProcess(command *exec.Cmd) {
	// Normal shutdown waits/reaps explicitly. Also stop an isolated worker if
	// its Linux supervisor dies before it can run that cleanup.
	command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}

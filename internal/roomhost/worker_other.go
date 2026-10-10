//go:build !linux

package roomhost

import "os/exec"

func configureWorkerProcess(*exec.Cmd) {}

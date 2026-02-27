package main

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/process"
)

func killProcess(pid int32) error {
	p, err := process.NewProcess(pid)
	if err != nil {
		return fmt.Errorf("unable to find PID %d: %w", pid, err)
	}
	if err := p.Kill(); err != nil {
		return fmt.Errorf("failed to terminate process %d: %w", pid, err)
	}
	return nil
}

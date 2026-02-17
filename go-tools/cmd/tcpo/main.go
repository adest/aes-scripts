package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"

	"go-tools/pkg/lib"
)

type ConnectionInfo struct {
	Proto       string
	LocalAddr   string
	Pid         int32
	ProcessName string
}

func main() {
	// 1. Retrieve all TCP network connections
	connections, err := getTcpListeningConnections()
	if err != nil {
		log.Fatalf("Error while retrieving connections: %v", err)
	}

	printConnections(connections)

	// 2. User interaction: choose to kill a process or quit
	shouldKill := askKillInput()
	if shouldKill {
		if len(connections) == 0 {
			fmt.Println("No listening processes found.")
			return
		}
		idx, err := fzfSelect(connections)
		if err != nil {
			if err == fuzzyfinder.ErrAbort {
				fmt.Println("Process selection cancelled.")
				return
			}
			lib.Exit(fmt.Errorf("Fuzzyfinder error: %v\n", err))
		}
		targetPid := connections[idx].Pid
		err = killProcess(targetPid)
		if err != nil {
			lib.Exit(fmt.Errorf("Error while killing process %d: %v", targetPid, err))
		} else {
			fmt.Printf("Process %d terminated successfully.\n", targetPid)
		}
	}
}

func getTcpListeningConnections() ([]ConnectionInfo, error) {
	connections, err := net.Connections("tcp")
	if err != nil {
		return nil, err
	}

	var listenConns []ConnectionInfo
	for _, conn := range connections {
		if conn.Status == "LISTEN" {
			pid := conn.Pid
			processName := "Unknown"
			if pid > 0 {
				p, err := process.NewProcess(pid)
				if err == nil {
					name, err := p.Name()
					if err == nil {
						processName = name
					}
				}
			}
			localAddr := fmt.Sprintf("%s:%d", conn.Laddr.IP, conn.Laddr.Port)
			listenConns = append(listenConns, ConnectionInfo{
				Proto:       "TCP",
				LocalAddr:   localAddr,
				Pid:         pid,
				ProcessName: processName,
			})
		}
	}
	return listenConns, nil
}

func printConnections(connections []ConnectionInfo) {
	fmt.Printf("%-10s %-25s %-10s %-20s\n", "PROTO", "LOCAL ADDRESS", "PID", "PROCESS NAME")
	fmt.Println("---------------------------------------------------------------------------")
	for _, conn := range connections {
		fmt.Printf("%-10s %-25s %-10d %-20s\n", conn.Proto, conn.LocalAddr, conn.Pid, conn.ProcessName)
	}
}

func fzfSelect(connections []ConnectionInfo) (int, error) {
	return fuzzyfinder.Find(
		connections,
		func(i int) string {
			return fmt.Sprintf("%s | %s | PID: %d | %s", connections[i].Proto, connections[i].LocalAddr, connections[i].Pid, connections[i].ProcessName)
		},
		fuzzyfinder.WithPromptString("Select a process to kill: "),
	)
}

// askKillInput prompts the user to type 'kill' to proceed with process selection or press Enter to quit
func askKillInput() bool {
	fmt.Print("\nType 'kill' to select a process to terminate, or press Enter to quit: ")
	var input string
	fmt.Scanln(&input)
	return strings.TrimSpace(strings.ToLower(input)) == "kill"
}

func killProcess(pid int32) error {
	// 1. Initialize the process
	p, err := process.NewProcess(pid)
	if err != nil {
		// Wrap the error to provide context
		return fmt.Errorf("unable to find PID %d: %w", pid, err)
	}

	// 2. Attempt to terminate
	if err := p.Kill(); err != nil {
		return fmt.Errorf("failed to terminate process %d: %w", pid, err)
	}

	return nil // Everything went fine
}

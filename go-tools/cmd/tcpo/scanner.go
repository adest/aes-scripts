package main

import (
	"fmt"
	"strings"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type ConnectionInfo struct {
	Proto       string
	LocalAddr   string
	Pid         int32
	ProcessName string
}

// ScanConnections returns listening connections for the given protocol.
// proto: "tcp", "udp", or "all".
func ScanConnections(proto string) ([]ConnectionInfo, error) {
	var kinds []string
	switch proto {
	case "udp":
		kinds = []string{"udp"}
	case "all":
		kinds = []string{"tcp", "udp"}
	default:
		kinds = []string{"tcp"}
	}

	var results []ConnectionInfo
	for _, kind := range kinds {
		conns, err := net.Connections(kind)
		if err != nil {
			return nil, err
		}
		for _, conn := range conns {
			isListening := conn.Status == "LISTEN" || (kind == "udp" && conn.Laddr.Port > 0)
			if !isListening {
				continue
			}
			pid := conn.Pid
			processName := "unknown"
			if pid > 0 {
				if p, err := process.NewProcess(pid); err == nil {
					if name, err := p.Name(); err == nil {
						processName = name
					}
				}
			}
			results = append(results, ConnectionInfo{
				Proto:       strings.ToUpper(kind),
				LocalAddr:   fmt.Sprintf("%s:%d", conn.Laddr.IP, conn.Laddr.Port),
				Pid:         pid,
				ProcessName: processName,
			})
		}
	}
	return results, nil
}

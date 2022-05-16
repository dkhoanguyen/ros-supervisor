package resolvable

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// TODO: We should only bind mount to 1 hosts file instead of the entire /etc dir
const pattern = "Custom-ROS-Hostname\n"

type HostFile struct {
	Path     string
	allLines []string
}
type Host struct {
	Ip       string
	Id       string
	Hostname string
}

func (hf *HostFile) PrepareFile() {
	f, err := os.Open(hf.Path)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		hf.allLines = append(hf.allLines, scanner.Text()+"\n")
	}
	// Remove lines with known patterns
	for idx, line := range hf.allLines {
		if strings.Contains(pattern, line) {
			hf.allLines = hf.allLines[:idx]
			break
		}
	}

	hf.allLines = append(hf.allLines, "\n\n"+pattern)
}

func (hf *HostFile) CleanUpFile() {
	// Remove lines with known patterns
	for idx, line := range hf.allLines {
		if strings.Contains(line, pattern) {
			hf.allLines = hf.allLines[:idx]
			break
		}
	}
	aux, err := os.Create(hf.Path)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer aux.Close()
	for _, line := range hf.allLines {
		aux.WriteString(line)
	}
	aux.Sync()

}

func (hf *HostFile) UpdateHostFile(hosts map[string]Host) {
	for name, host := range hosts {
		line := fmt.Sprintf("%s    %s   %s   %s\n", host.Ip, name, host.Id, host.Hostname)
		hf.allLines = append(hf.allLines, line)
	}

	aux, err := os.Create(hf.Path)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer aux.Close()
	for _, line := range hf.allLines {
		aux.WriteString(line)
	}
	aux.Sync()
}

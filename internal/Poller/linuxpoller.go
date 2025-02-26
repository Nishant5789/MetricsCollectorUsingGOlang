package Poller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type MetricResult struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Error string `json:"error,omitempty"`
}

type OverallResult struct {
	Metrics []MetricResult `json:"metrics"`
}

func GetLinuxDeviceData(username, password, host, port string) string {
	commands := map[string]string{
	"system.overall.memory.free.bytes":    "free -b | awk '/Mem:/ {print $4}'",
	"system.memory.free.bytes":            "free -b | awk '/Mem:/ {print $4}'", 
	"system.overall.memory.used.bytes":    "free -b | awk '/Mem:/ {print $3}'",
	"system.memory.used.bytes":            "free -b | awk '/Mem:/ {print $3}'",
	"system.memory.installed.bytes":       "free -b | awk '/Mem:/ {print $2}'",
	"system.memory.available.bytes":       "free -b | awk '/Mem:/ {print $7}'",
	"system.cache.memory.bytes":           "free -b | awk '/Mem:/ {print $6}'",
	"system.buffer.memory.bytes":          "free -b | awk '/Mem:/ {print $6}'", 
	"system.overall.memory.used.percent":  "free -b | awk '/Mem:/ {print ($3/$2)*100}'",
	"system.memory.used.percent":          "free -b | awk '/Mem:/ {print ($3/$2)*100}'", 
	"system.overall.memory.free.percent":  "free -b | awk '/Mem:/ {print ($4/$2)*100}'",
	"system.memory.free.percent":          "free -b | awk '/Mem:/ {print ($4/$2)*100}'", 
	"system.swap.memory.free.bytes":       "free -b | awk '/Swap:/ {print $4}'",
	"system.swap.memory.used.bytes":       "free -b | awk '/Swap:/ {print $3}'",
	"system.swap.memory.used.percent":     "free -b | awk '/Swap:/ {if ($2 > 0) print ($3/$2)*100; else print 0}'",
	"system.swap.memory.free.percent":     "free -b | awk '/Swap:/ {if ($2 > 0) print ($4/$2)*100; else print 0}'",
	"system.load.avg1.min":                "uptime | awk '{print $8}' | tr -d ','",
	"system.load.avg5.min":                "uptime | awk '{print $9}' | tr -d ','",
	"system.load.avg15.min":               "uptime | awk '{print $10}' | tr -d ','",
	"system.cpu.cores":                    "nproc",
	"system.cpu.percent":                  "top -bn1 | awk '/%Cpu/ {print 100 - $8}'",
	"system.cpu.kernel.percent":           "cat /proc/stat | awk '/cpu / {print ($2+$4)*100/($2+$4+$5)}'",
	"system.cpu.idle.percent":             "cat /proc/stat | awk '/cpu / {print $5*100/($2+$4+$5)}'",
	"system.cpu.interrupt.percent":        "cat /proc/stat | awk '/cpu / {print $7*100/($2+$4+$5)}'",
	"system.cpu.io.percent":               "iostat -c | awk 'NR==4 {print $4}'", // Avoids delay
	"system.disk.capacity.bytes":          "df -B1 / | awk 'NR==2 {print $2}'",
	"system.disk.free.bytes":              "df -B1 / | awk 'NR==2 {print $4}'",
	"system.disk.used.bytes":              "df -B1 / | awk 'NR==2 {print $3}'",
	"system.disk.free.percent":            "df -h / | awk 'NR==2 {print $4}' | tr -d '%'",
	"system.disk.used.percent":            "df -h / | awk 'NR==2 {print $5}' | tr -d '%'",
	"system.network.tcp.connections":      "ss -t | wc -l",
	"system.network.udp.connections":      "ss -u | wc -l",
	"system.network.error.packets":        "cat /proc/net/dev | awk 'NR>2 {sum += $4+$12} END {print sum}'",
	"system.running.processes":            "ps -e | wc -l",
	"system.blocked.processes":            "ps -e -o stat | grep 'D' | wc -l",
	"system.threads":                      "cat /proc/stat | awk '/processes/ {print $2}'", // Total forks, approx thread count
	"system.os.name":                      "uname -s",
	"system.os.version":                   "lsb_release -rs 2>/dev/null || echo 'N/A'",
	"system.name":                         "hostname",
	"started.time":                        "uptime -p | cut -d' ' -f2-",
	"started.time.seconds":                "cat /proc/uptime | awk '{print $1}'",
	"system.context.switches.per.sec":     "cat /proc/stat | awk '/ctxt/ {print $2}'", // Cumulative, not per-second
	}

	client, err := connectToSSH(username, password, host, port)
	if err != nil {
		log.Printf("SSH connection failed: %v", err)
		return string("SSH connection failed")
	}
	log.Printf("SSH connection Success")
	defer client.Close()

	jobs := make(chan [2]string, len(commands))
	results := make(chan MetricResult, len(commands))
	var wg sync.WaitGroup

	go worker(client, jobs, results, &wg)

	for metric, cmd := range commands {
		wg.Add(1) 
		jobs <- [2]string{metric, cmd}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var metrics []MetricResult
	for result := range results {
		metrics = append(metrics, result)
		if result.Error != "" {
			log.Printf("Error: %s - %s", result.Name, result.Error)
		} 
	}

	overallResult := OverallResult{Metrics: metrics}
	jsonData, err := json.MarshalIndent(overallResult, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal JSON: %v", err)
		return string("Failed to marshal JSON")
	}

    return string(jsonData)
}

// Connect to the SSH server
func connectToSSH(user, password, host, port string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host, port), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect via SSH: %v", err)
	}
	return client, nil
}

// Execute a command on the remote server
func executeCommand(client *ssh.Client, metricName, cmd string, results chan<- MetricResult, wg *sync.WaitGroup) {
	defer wg.Done()

	session, err := client.NewSession()
	if err != nil {
		results <- MetricResult{Name: metricName, Value: "N/A", Error: fmt.Sprintf("SSH session failed: %v", err)}
		return
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		results <- MetricResult{Name: metricName, Value: "N/A", Error: fmt.Sprintf("Command failed: %v | Stderr: %s", err, stderr.String())}
		return
	}

	results <- MetricResult{Name: metricName, Value: stdout.String()}
}

// Worker function to process jobs
func worker(client *ssh.Client, jobs <-chan [2]string, results chan<- MetricResult, wg *sync.WaitGroup) {
	for job := range jobs {
		metricName, cmd := job[0], job[1]
		executeCommand(client, metricName, cmd, results, wg)
	}
}



package Poller

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	// Define commands to collect metrics
	commands := map[string]string{
		"system.overall.memory.free.bytes":     "free -b | grep 'Mem:' | awk '{print $4}'",
		"system.load.avg15.min":                "uptime | awk '{print $10}'",
		"system.swap.memory.free.bytes":        "free -b | grep 'Swap:' | awk '{print $4}'",
		"system.swap.memory.used.percent":      "free -b | grep 'Swap:' | awk '{if ($2 > 0) print ($3/$2)*100; else print 0}'",
		"system.load.avg1.min":                 "uptime | awk '{print $8}'",
		"system.network.udp.connections":       "netstat -ua | grep 'udp' | wc -l",
		"system.load.avg5.min":                 "uptime | awk '{print $9}'",
		"system.blocked.processes":             "ps -eLf | grep ' D ' | wc -l",
		"system.cache.memory.bytes":            "free -b | grep 'Mem:' | awk '{print $6}'",
		"system.network.tcp.connections":       "netstat -ta | grep 'tcp' | wc -l",
		"system.cpu.cores":                     "lscpu | grep '^CPU(s):' | awk '{print $2}'",
		"system.os.name":                       "uname -s",
		"system.os.version":                    "lsb_release -r | grep 'Release' | awk '{print $2}'",
		"system.context.switches.per.sec":      "vmstat 1 2 | tail -1 | awk '{print $12}'",
		"system.disk.capacity.bytes":           "df -B1 | tail -1 | awk '{print $2}'",
		"system.buffer.memory.bytes":           "free -b | grep 'Mem:' | awk '{print $6}'",
		"system.swap.memory.used.bytes":        "free -b | grep 'Swap:' | awk '{print $3}'",
		"system.cpu.interrupt.percent":         "mpstat 1 1 | grep 'Average' | awk '{print $8}'",
		"system.memory.available.bytes":        "free -b | grep 'Mem:' | awk '{print $7}'",
		"system.overall.memory.used.bytes":     "free -b | grep 'Mem:' | awk '{print $3}'",
		"started.time":                         "uptime | awk '{print $1}'",
		"started.time.seconds":                 "awk '{print $1}' /proc/uptime",
		"system.swap.memory.free.percent":      "free -b | grep 'Swap:' | awk '{if ($2 > 0) print ($4/$2)*100; else print 0}'",
		"system.memory.installed.bytes":        "free -b | grep 'Mem:' | awk '{print $2}'",
		"system.cpu.percent":                   "top -bn1 | grep '%Cpu' | awk '{print 100 - $8}'",
		"system.disk.free.bytes":               "df -B1 | tail -1 | awk '{print $4}'",
		"system.memory.used.bytes":             "free -b | grep 'Mem:' | awk '{print $3}'",
		"system.memory.free.bytes":             "free -b | grep 'Mem:' | awk '{print $4}'",
		"system.overall.memory.used.percent":   "free -b | grep 'Mem:' | awk '{print ($3/$2)*100}'",
		"system.running.processes":             "ps aux | wc -l | awk '{print $1}'",
		"system.memory.free.percent":           "free -b | grep 'Mem:' | awk '{print ($4/$2)*100}'",
		"system.disk.free.percent":             "df -h / | tail -n1 | awk '{print $4}' | grep -o '[0-9]*'",
		"system.cpu.io.percent":                "iostat -c 1 2 | tail -n2 | head -n1 | awk '{print $4}'",
		"system.disk.used.percent":             "df -h / | tail -n1 | awk '{print $5}' | grep -o '[0-9]*'",
		"system.network.error.packets":         "netstat -i | tail -n+3 | awk '{print $6+$8}' | grep -v '^0$' | wc -l",
		"system.threads":                       "ps -eLf | wc -l | awk '{print $1-1}'",
		"system.name":                          "uname -n | awk '{print $1}'",
		"system.disk.used.bytes":               "df -B1 / | tail -n1 | awk '{print $3}'",
		"system.memory.used.percent":           "free -b | grep 'Mem:' | awk '{print ($3/$2)*100}'",
		"system.overall.memory.free.percent":   "free -b | grep 'Mem:' | awk '{print ($4/$2)*100}'",
		"system.cpu.kernel.percent":            "mpstat 1 1 | grep 'Average' | awk '{print $3}'",
		"system.cpu.idle.percent":              "mpstat 1 1 | grep 'Average' | awk '{print $11}'",
	}

	// Connect to SSH
	client, err := connectToSSH(username, password, host, port)
	if err != nil {
		fmt.Printf("❌ SSH connection failed: %v\n", err)
		return string("")
	}
	defer client.Close()

	// Channels and wait group
	jobs := make(chan [2]string, len(commands))
	results := make(chan MetricResult, len(commands))
	var wg sync.WaitGroup

	// Start workers
	numWorkers := 5
	for i := 0; i < numWorkers; i++ {
		go worker(client, jobs, results, &wg)
	}

	// Send commands to the jobs channel and increment wait group
	for metric, cmd := range commands {
		wg.Add(1) // Increment wait group before sending job
		jobs <- [2]string{metric, cmd}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var metrics []MetricResult
	for result := range results {
		metrics = append(metrics, result)
		if result.Error != "" {
			fmt.Printf("⚠️ Error: %s - %s\n", result.Name, result.Error)
		} else {
			fmt.Printf("✅ %s: %s\n", result.Name, result.Value)
		}
	}

	// Write results to JSON
	overallResult := OverallResult{Metrics: metrics}
	jsonData, err := json.MarshalIndent(overallResult, "", "  ")
	if err != nil {
		fmt.Printf("❌ Failed to marshal JSON: %v\n", err)
		return string("")
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



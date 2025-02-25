package main

import (
	"fmt"
	"github.com/Nishant5789/LinuxMetricsCollectorUsingGOlang/internal/Poller"
)

func main() {
	username := "nishant1290"
	password := "nushrat!@#"
	host := "192.168.1.28"
	port := "22"

	jsonResponse := Poller.GetLinuxDeviceData(username,password, host, port)
	fmt.Println(jsonResponse)
	
}
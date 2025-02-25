package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
	"github.com/Nishant5789/LinuxMetricsCollectorUsingGOlang/internal/Poller"
	"github.com/pebbe/zmq4"
)

type RequestPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
}

func main() {
	responder, err := zmq4.NewSocket(zmq4.REP)
	if err != nil {
		log.Fatal(err)
	}
	defer responder.Close()

	responder.Bind("tcp://127.0.0.1:5555") // Bind to port 5555
	fmt.Println("ZMQ Server is running on port 5555...")

	for {
		request, err := responder.Recv(0)
		if err != nil {
			log.Println("Error receiving request:", err)
			continue
		}

		// fmt.Println("Received request:", request)

		var reqPayload RequestPayload
		err = json.Unmarshal([]byte(request), &reqPayload)
		if err != nil {
			log.Println("Error parsing request JSON:", err)
			continue
		}

		time.Sleep(2 * time.Second)

		jsonResponse := Poller.GetLinuxDeviceData(reqPayload.Username, reqPayload.Password, reqPayload.Host, reqPayload.Port)
		fmt.Println("Sending response:\n", jsonResponse)

		_, err = responder.Send(jsonResponse, 0)
		if err != nil {
			log.Println("Error sending response:", err)
		}
	}	
}
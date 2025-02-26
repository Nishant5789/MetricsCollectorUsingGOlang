package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Nishant5789/LinuxMetricsCollectorUsingGOlang/internal/Poller"
	"github.com/panjf2000/ants/v2"
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

	if err := responder.Bind("tcp://127.0.0.1:5555"); err != nil {
		log.Fatal("Failed to bind ZMQ socket:", err)
	}
	fmt.Println("ZMQ Server is running on port 5555...")

	pool, err := ants.NewPool(10, ants.WithMaxBlockingTasks(10), ants.WithExpiryDuration(60*time.Second), ants.WithNonblocking(false))
	if err != nil {
		log.Fatal("Failed to create thread pool:", err)
	}
	defer pool.Release()

	var taskID int
	var taskMu sync.Mutex

	for {
		request, err := responder.Recv(0)
		if err != nil {
			log.Println("Error receiving request:", err)
			continue
		}

		var reqPayload RequestPayload
		if err := json.Unmarshal([]byte(request), &reqPayload); err != nil {
			log.Println("Error parsing request JSON:", err)
			continue
		}

		taskMu.Lock()
		currentTaskID := taskID
		taskID++
		taskMu.Unlock()

		var wg sync.WaitGroup
		wg.Add(1)

		err = pool.Submit(func() {
			defer wg.Done()
			jsonResponse := Poller.GetLinuxDeviceData(reqPayload.Username, reqPayload.Password, reqPayload.Host, reqPayload.Port)
			log.Printf("Task %d executed with response: %s", currentTaskID, jsonResponse)

			_, err := responder.Send(jsonResponse, 0)
			if err != nil {
				log.Printf("Task %d error sending response: %v", currentTaskID, err)
			}
		})
		if err != nil {
			log.Printf("Task %d rejected, running in caller", currentTaskID)
			go func(id int) {
				defer wg.Done()
				jsonResponse := Poller.GetLinuxDeviceData(reqPayload.Username, reqPayload.Password, reqPayload.Host, reqPayload.Port)
				log.Printf("Task %d executed by caller with response: %s", id, jsonResponse)

				_, err := responder.Send(jsonResponse, 0)
				if err != nil {
					log.Printf("Task %d error sending response in caller: %v", id, err)
				}
			}(currentTaskID)
		}
		wg.Wait()
	}
}
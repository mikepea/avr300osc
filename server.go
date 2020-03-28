package main

import (
	"fmt"
	"github.com/enriquebris/goconcurrentqueue"
	"github.com/hypebeast/go-osc/osc"
	"time"
)

type OscEvent struct {
	TimeReceived time.Time
	OscMessage   *osc.Message
}

/*
func queueConsumer(queue goconcurrentqueue.Queue)) {
}
*/

func main() {

	addr := "0.0.0.0:8765"
	var (
		fifo = goconcurrentqueue.NewFIFO()
	)
	go func() {
		fmt.Println("1 - Waiting for next enqueued element")
		for {
			value, err := fifo.DequeueOrWaitForNextElement()
			if err != nil {
				break
			}
			fmt.Printf("2 - Dequeued element: %v\n", value)
		}
	}()

	d := osc.NewStandardDispatcher()
	d.AddMsgHandler("*", func(msg *osc.Message) {
		fmt.Printf("Enqueuing: ")
		osc.PrintMessage(msg)
		fifo.Enqueue(OscEvent{time.Now(), msg})
	})

	server := &osc.Server{
		Addr:       addr,
		Dispatcher: d,
	}

	server.ListenAndServe()

}

package main

import (
	"fmt"
	"github.com/enriquebris/goconcurrentqueue"
	"github.com/hypebeast/go-osc/osc"
	"github.com/mikepea/avr300osc/arcamctl"
	"time"
)

type OscEvent struct {
	TimeReceived time.Time
	OscMessage   *osc.Message
}

func queueConsumer(queue goconcurrentqueue.Queue) {
	for {
		value, err := queue.DequeueOrWaitForNextElement()
		if err != nil {
			break
		}
		handleQueueElement(value)
	}
}

func handleAmpVolume(o OscEvent) {
	fmt.Printf("Handling Amp Volume: %v\n", o.OscMessage.Arguments)
}

func handleAmpPower(o OscEvent) {
	fmt.Printf("Handling Amp Power: %v\n", o.OscMessage.Arguments)
	arcamctl.PowerOn()
}

func handleOscEvent(o OscEvent) {
	address := o.OscMessage.Address
	if address == "/clean__avr_amp__power" {
		handleAmpPower(o)
	} else if address == "/clean__avr_amp__volume" {
		handleAmpVolume(o)
	} else {
		fmt.Printf("Unknown OSC address: %s\n - value %v", address, o.OscMessage.Arguments)
	}
}

func handleQueueElement(i interface{}) {
	switch v := i.(type) {
	case OscEvent:
		handleOscEvent(v)
	default:
		fmt.Printf("Dequeued unknown element of type %T: %v\n", v, v)
	}
}

func main() {

	addr := "0.0.0.0:8765"
	var (
		fifo = goconcurrentqueue.NewFIFO()
	)
	go queueConsumer(fifo)

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

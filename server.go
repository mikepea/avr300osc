package main

import (
	"github.com/enriquebris/goconcurrentqueue"
	"github.com/hypebeast/go-osc/osc"
	"github.com/mikepea/avr300osc/arcamctl"
	"log"
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
	// Argument is a float from 0 to 1
	// convert to an int from 0 to 100
	volume := int(o.OscMessage.Arguments[0].(float32) * 100)
	arcamctl.VolumeSet(volume)
}

func handleAmpAudioSource(o OscEvent) {
	s := o.OscMessage.Arguments[0].(int32)
	switch s {
	case 0:
		arcamctl.AudioSelectSat()
	case 1:
		arcamctl.AudioSelectAux()
	case 2:
		arcamctl.AudioSelectCD()
	default:
		log.Printf("handleAmpAudioSource: unknown source %d", s)
	}
}

func handleAmpPowerOn(o OscEvent) {
	arcamctl.PowerOn()
}

func handleAmpPowerOff(o OscEvent) {
	arcamctl.PowerOff()
}

func handleOscEvent(o OscEvent) {
	address := o.OscMessage.Address
	if address == "/clean__avr_amp__power_on" {
		handleAmpPowerOn(o)
	} else if address == "/clean__avr_amp__power_off" {
		handleAmpPowerOff(o)
	} else if address == "/clean__avr_amp__mute" {
		arcamctl.Mute()
	} else if address == "/clean__avr_amp__unmute" {
		arcamctl.Unmute()
	} else if address == "/clean__avr_amp__volume" {
		handleAmpVolume(o)
	} else if address == "/clean__avr_amp__source" {
		handleAmpAudioSource(o)
	} else {
		log.Printf("Unknown OSC address: %s\n - value %v", address, o.OscMessage.Arguments)
	}
}

func handleQueueElement(i interface{}) {
	switch v := i.(type) {
	case OscEvent:
		handleOscEvent(v)
	default:
		log.Printf("Dequeued unknown element of type %T: %v\n", v, v)
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
		log.Printf("Enqueuing: ")
		osc.PrintMessage(msg)
		fifo.Enqueue(OscEvent{time.Now(), msg})
	})

	server := &osc.Server{
		Addr:       addr,
		Dispatcher: d,
	}

	server.ListenAndServe()

}

package main

import (
	"github.com/enriquebris/goconcurrentqueue"
	"github.com/hypebeast/go-osc/osc"
	"github.com/mikepea/avr300osc/arcamctl"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"time"
)

type OscEvent struct {
	TimeReceived time.Time
	OscMessage   *osc.Message
}

var a *arcamctl.ArcamAVRController

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
	a.VolumeSet(volume)
}

func handleAmpAudioSource(o OscEvent) {
	s := o.OscMessage.Arguments[0].(int32)
	switch s {
	case 0:
		a.AudioSelectSat()
	case 1:
		a.AudioSelectAux()
	case 2:
		a.AudioSelectCD()
	default:
		log.Printf("handleAmpAudioSource: unknown source %d", s)
	}
}

func handleAmpPowerOn(o OscEvent) {
	a.PowerOn()
}

func handleAmpPowerOff(o OscEvent) {
	a.PowerOff()
}

func handleOscEvent(o OscEvent) {
	address := o.OscMessage.Address
	if address == "/clean__avr_amp__power_on" {
		handleAmpPowerOn(o)
	} else if address == "/clean__avr_amp__power_off" {
		handleAmpPowerOff(o)
	} else if address == "/clean__avr_amp__mute" {
		a.Mute()
	} else if address == "/clean__avr_amp__unmute" {
		a.Unmute()
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

func prometheusExporterUpdate(a *arcamctl.ArcamAVRController) {
	volumeGauge.Set(float64(a.State.Zone1Volume))
}

func ampStateSender(a *arcamctl.ArcamAVRController) {
	client := osc.NewClient("192.168.131.175", 8080)
	for {
		prometheusExporterUpdate(a)
		msg := osc.NewMessage("/clean__avr_amp__test_slider")
		msg.Append(float32(a.State.Zone1Volume) / 100)
		client.Send(msg)
		time.Sleep(2 * time.Second)
	}
}

var (
	volumeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "amp_volume",
		Help: "Current Amp Volume (db) 0-100",
	})
)

func init() {
	prometheus.MustRegister(volumeGauge)
}

func main() {

	addr := "0.0.0.0:8765"
	var (
		fifo = goconcurrentqueue.NewFIFO()
	)
	go queueConsumer(fifo)

	var err error
	a, err = arcamctl.NewArcamAVRController()
	if err != nil {
		log.Fatalf("Could not initialize controller: %s", err)
	}
	go ampStateSender(a)

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

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":8080", nil)

	server.ListenAndServe()

}

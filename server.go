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
	// janky hack so i reduce the likelihood of a high volume
	// surprise in the morning
	if a.State.Zone1Volume > 30 {
		a.VolumeSet(30)
	}
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
	} else if address == "/clean__avr_amp__forcequit" {
		// bail (and let systemd restart us)
		log.Fatalf("OSC forcequit received: Quitting.")
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

func boolToFloat64(v bool) float64 {
	if v {
		return 1
	} else {
		return 0
	}
}

func prometheusExporterUpdate(a *arcamctl.ArcamAVRController) {
	volumeGauge.Set(float64(a.State.Zone1Volume))
	muteOnGauge.Set(boolToFloat64(a.State.Zone1MuteOn))
	poweredOnGauge.Set(boolToFloat64(a.State.PoweredOn))
	audioSourceGauge.Set(float64(a.State.Zone1AudioSource))
	serialFifoSizeGauge.Set(float64(a.State.SerialWriterQueueLength))
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
	poweredOnGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "amp_power_on_status",
		Help: "Current Amp Powered On State (0 standby, 1 on)",
	})
	muteOnGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "amp_mute_on_status",
		Help: "Current Amp Mute State (0 off, 1 on)",
	})
	audioSourceGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "amp_audio_source_status",
		Help: "Current Amp Audio Source [0:DVD, 1:SAT, ..., 8:DVDA]",
	})
	oscFifoSizeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "osc_fifo_size",
		Help: "Length of inbound OSC message queue",
	})
	serialFifoSizeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "serial_fifo_size",
		Help: "Length of Amp RS232 Writer message queue",
	})
)

func init() {
	prometheus.MustRegister(volumeGauge)
	prometheus.MustRegister(poweredOnGauge)
	prometheus.MustRegister(muteOnGauge)
	prometheus.MustRegister(audioSourceGauge)
	prometheus.MustRegister(oscFifoSizeGauge)
	prometheus.MustRegister(serialFifoSizeGauge)
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
		oscFifoSizeGauge.Set(float64(fifo.GetLen()))
	})

	server := &osc.Server{
		Addr:       addr,
		Dispatcher: d,
	}

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":8080", nil)

	server.ListenAndServe()

}

package main

import (
	"fmt"
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

func handleAmpVolume(o OscEvent, zone int) {
	// Argument is a float from 0 to 1
	// convert to an int from 0 to 100
	volume := int(o.OscMessage.Arguments[0].(float32) * 100)
	a.VolumeSet(volume, zone)
}

func handleAmpAudioSource(o OscEvent, zone int) {
	s := o.OscMessage.Arguments[0].(int32)
	switch s {
	case 0:
		a.AudioSelectSat(zone)
	case 1:
		a.AudioSelectAux(zone)
	case 2:
		a.AudioSelectCD(zone)
	default:
		log.Printf("handleAmpAudioSource: unknown source %d", s)
	}
}

func handleAmpPowerOff(o OscEvent) {
	// janky hack so i reduce the likelihood of a high volume
	// surprise in the morning
	if a.State.Zone1Volume > 30 {
		a.VolumeSet(30, 1)
	}
	if a.State.Zone2Volume > 30 {
		a.VolumeSet(30, 2)
	}
	a.PowerOff(1)
	a.PowerOff(2)
}

func handleOscEvent(o OscEvent) {
	address := o.OscMessage.Address
	if address == "/clean__avr_amp__power_on" {
		a.PowerOn(1)
	} else if address == "/clean__avr_amp__power_on_2" {
		a.PowerOn(2)
	} else if address == "/clean__avr_amp__power_off" {
		handleAmpPowerOff(o)
	} else if address == "/clean__avr_amp__mute" {
		a.Mute(1)
	} else if address == "/clean__avr_amp__mute1" {
		a.Mute(1)
	} else if address == "/clean__avr_amp__mute2" {
		a.Mute(2)
	} else if address == "/clean__avr_amp__unmute" {
		a.Unmute(1)
	} else if address == "/clean__avr_amp__unmute1" {
		a.Unmute(1)
	} else if address == "/clean__avr_amp__unmute3" {
		a.Unmute(3)
	} else if address == "/clean__avr_amp__volume" {
		handleAmpVolume(o, 1)
	} else if address == "/clean__avr_amp__volume1" {
		handleAmpVolume(o, 1)
	} else if address == "/clean__avr_amp__volume2" {
		handleAmpVolume(o, 2)
	} else if address == "/clean__avr_amp__source" {
		handleAmpAudioSource(o, 1)
	} else if address == "/clean__avr_amp__source1" {
		handleAmpAudioSource(o, 1)
	} else if address == "/clean__avr_amp__source2" {
		handleAmpAudioSource(o, 2)
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
	poweredOnGauge.Set(boolToFloat64(a.State.Zone1PoweredOn))
	audioSourceGauge.Set(float64(a.State.Zone1AudioSource))
	serialFifoSizeGauge.Set(float64(a.State.SerialWriterQueueLength))
}

func sendOscVolume(client *osc.Client, a *arcamctl.ArcamAVRController) {
	msg := osc.NewMessage("/clean__avr_amp__volume")
	msg.Append(float32(a.State.Zone1Volume) / 100)
	client.Send(msg)
}

func sendOscVolumeZone2(client *osc.Client, a *arcamctl.ArcamAVRController) {
	msg := osc.NewMessage("/clean__avr_amp__volume2")
	msg.Append(float32(a.State.Zone2Volume) / 100)
	client.Send(msg)
}

func translateAudioSourceToOscVal(a int) int {
	if a == 1 {
		return 0 // SAT
	}
	if a == 3 {
		return 1 // AUX/PVR
	}
	if a == 5 {
		return 2 // CD
	}
	return -1
}

func sendOscAudioSource(client *osc.Client, a *arcamctl.ArcamAVRController) {
	msg := osc.NewMessage("/clean__avr_amp__source")
	ampSource := a.State.Zone1AudioSource
	if s := translateAudioSourceToOscVal(ampSource); s >= 0 {
		msg.Append(float32(s))
		client.Send(msg)
	}
}

func sendOscTextStatus(client *osc.Client, a *arcamctl.ArcamAVRController) {
	msg := osc.NewMessage("/clean__avr_amp__status")
	v1 := a.State.Zone1Volume
	v2 := a.State.Zone2Volume
	s1 := a.State.Zone1AudioSource
	s2 := a.State.Zone2AudioSource
	p1 := 0
	p2 := 0
	if a.State.Zone1PoweredOn {
		p1 = 1
	}
	if a.State.Zone2PoweredOn {
		p2 = 1
	}
	statusMsg := fmt.Sprintf("P:%d/%d V%d/%d S:%d/%d", p1, p2, v1, v2, s1, s2)
	msg.Append(statusMsg)
	log.Printf("StatusMsg: %s\n", statusMsg)
	client.Send(msg)
}

func ampStateSender(a *arcamctl.ArcamAVRController) {
	client := osc.NewClient("192.168.131.175", 8080)
	for {
		prometheusExporterUpdate(a)
		sendOscVolume(client, a)
		sendOscVolumeZone2(client, a)
		sendOscAudioSource(client, a)
		sendOscTextStatus(client, a)
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

package arcamctl

import (
	"github.com/enriquebris/goconcurrentqueue"
	"github.com/tarm/serial"
	"log"
	"strings"
	"time"
)

const SERIAL_DEV = "/dev/ttyUSB0"
const SERIAL_BAUD = 38400

var s *serial.Port

type ArcamAmpState struct {
	SerialWriterQueueLength int
	Zone1Volume             int
	PoweredOn               bool
	Zone1MuteOn             bool
	Zone1AudioSource        int
}

type ArcamAVRController struct {
	State      *ArcamAmpState
	serialPort *serial.Port
	writeFifo  *goconcurrentqueue.FIFO
}

func NewArcamAVRController() (*ArcamAVRController, error) {
	log.Println("init: Opening port")
	c := &serial.Config{Name: SERIAL_DEV, Baud: SERIAL_BAUD}
	s, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}
	fifo := goconcurrentqueue.NewFIFO()
	a := &ArcamAVRController{
		State:      &ArcamAmpState{},
		serialPort: s,
		writeFifo:  fifo,
	}
	go a.serialReader()
	go a.serialWriter()
	go a.statusPoller()
	return a, nil
}

func (a *ArcamAVRController) handleStatusMessages(msgs []string) {
	for _, msg := range msgs {
		a.handleStatusMessage(msg)
	}
}

/*
*  AV_0 - volume set
*  AV_* - power
*  AV_. - mute
*  AV_/ - volume inc/dec/status
*  AV_1 - source select
 */
func (a *ArcamAVRController) handleStatusMessage(msg string) {
	if len(msg) < 4 {
		log.Printf("message too short: %v\n", msg)
		return
	}

	family := msg[:4]
	switch family {
	case "AV_0":
		a.handleVolumeSetStatus(msg)
	case "AV_/":
		a.handleVolumeChangeStatus(msg)
	case "AV_*":
		a.handlePowerStatus(msg)
	case "AV_.":
		a.handleMuteStatus(msg)
	case "AV_1":
		a.handleSourceStatus(msg)
	default:
		log.Printf("unhandled message family: %s\n", family)
	}
}

func checkStatusMessage(msg string, header string, length int) bool {
	if msg[:4] != header {
		log.Printf("Invalid message: %s", msg)
		return false
	}
	msgStatus := string(msg[4])
	if msgStatus == "R" {
		return false // R is ok to silently fail on
	}
	if msgStatus != "P" {
		log.Printf("Invalid message: %s", msg)
		return false
	}
	if len(msg) != length {
		log.Printf("Invalid message (wrong length): %s", msg)
		return false
	}
	return true
}

func (a *ArcamAVRController) handleVolumeChangeStatus(msg string) {
	if !checkStatusMessage(msg, "AV_/", 7) {
		return
	}
	a.handleVolumeStatus(msg)
}

func (a *ArcamAVRController) handleVolumeSetStatus(msg string) {
	if !checkStatusMessage(msg, "AV_0", 7) {
		return
	}
	a.handleVolumeStatus(msg)
}

func (a *ArcamAVRController) handleVolumeStatus(msg string) {
	zone := byte(msg[5])
	val := byte(msg[6])
	if zone != 0x31 {
		log.Printf("Zone2 not handled: %s", msg)
	}
	volume := int(val - 0x30)
	a.State.Zone1Volume = volume // TODO this needs to be concurrency safe

}

func (a *ArcamAVRController) handlePowerStatus(msg string) {
	if !checkStatusMessage(msg, "AV_*", 7) {
		return
	}
	zone := byte(msg[5])
	val := byte(msg[6])
	if zone != 0x31 {
		log.Printf("Zone2 not handled: %s", msg)
	}
	if val == 0x31 {
		a.State.PoweredOn = true
	} else {
		a.State.PoweredOn = false // standby, or off.
	}
}

func (a *ArcamAVRController) handleMuteStatus(msg string) {
	if checkStatusMessage(msg, "AV_.", 7) == false {
		return
	}
	zone := byte(msg[5])
	val := byte(msg[6])
	if zone != 0x31 {
		log.Printf("Zone2 not handled: %s", msg)
	}
	if val == 0x30 {
		a.State.Zone1MuteOn = true
	} else {
		a.State.Zone1MuteOn = false
	}
}

func (a *ArcamAVRController) handleSourceStatus(msg string) {
	if checkStatusMessage(msg, "AV_1", 7) == false {
		return
	}
	zone := byte(msg[5])
	val := byte(msg[6])
	if zone != 0x31 {
		log.Printf("Zone2 not handled: %s", msg)
	}
	// 0: 0x30 DVD
	// 1: 0x31 SAT
	// 2: 0x32 AV
	// 3: 0x33 PVR
	// 4: 0x34 VCR
	// 5: 0x35 CD
	// 6: 0x36 FM
	// 7: 0x37 AM
	// 8: 0x38 DVDA
	a.State.Zone1AudioSource = int(val) - 0x30
}

func (a *ArcamAVRController) serialReader() {
	log.Println("Setting up port reader")
	var msgOverrun []byte
	for {

		buf := make([]byte, 128)
		n, err := a.serialPort.Read(buf)
		if err != nil {
			log.Fatal(err)
		}

		totalMsg := string(msgOverrun) + string(buf[:n])
		msgs := strings.Split(totalMsg, "\r")

		if msgs[len(msgs)-1] == "" {
			msgOverrun = []byte(``)
		} else {
			// end of buffer was not a complete message terminated with \r
			msgOverrun = []byte(msgs[len(msgs)-1])
		}

		// last element is either empty or an incomplete buffer, strip it
		msgs = msgs[:len(msgs)-1]

		a.handleStatusMessages(msgs)

	}
}

func (a *ArcamAVRController) serialWriter() {
	for {
		value, err := a.writeFifo.DequeueOrWaitForNextElement()
		if err != nil {
			log.Fatalf("Error reading from writer queue: %s", err)
		}
		msg := value.([]byte)
		_, err = a.serialPort.Write(msg)
		if err != nil {
			log.Printf("Failed to write value '%s', error: %d", msg, err)
		}
	}
}

func (a *ArcamAVRController) QueueWrite(msg []byte) {
	a.writeFifo.Enqueue(msg)
}

func (a *ArcamAVRController) QueueStatusPoller() {
	a.State.SerialWriterQueueLength = a.writeFifo.GetLen()
	time.Sleep(1 * time.Second)
}

func (a *ArcamAVRController) statusPoller() {
	for {
		a.PowerStatus()
		if a.State.PoweredOn {
			// these return an 'R' code if amp is on standby,
			// so no point in polling if we aren't on.
			a.MuteStatus()
			a.VolumeStatus()
			a.AudioSelectStatus()
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (a *ArcamAVRController) PowerOn() {
	log.Println("PowerOn called")
	msg := []byte("PC_*11\r")
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) PowerOff() {
	log.Println("PowerOff called")
	msg := []byte("PC_*10\r")
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) PowerStatus() {
	msg := []byte("PC_*19\r")
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) Mute() {
	log.Println("Mute called")
	msg := []byte("PC_.10\r")
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) Unmute() {
	log.Println("Unmute called")
	msg := []byte("PC_.11\r")
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) MuteStatus() {
	msg := []byte("PC_.19\r")
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) AudioSelectStatus() {
	msg := []byte("PC_119\r")
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) AudioSelectSat() {
	log.Println("AudioSelectSat called")
	msg := []byte("PC_111\r")
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) AudioSelectPVR() {
	a.AudioSelectAux()
}

func (a *ArcamAVRController) AudioSelectAux() {
	log.Println("AudioSelectAux called")
	msg := []byte("PC_113\r")
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) AudioSelectCD() {
	log.Println("AudioSelectCD called")
	msg := []byte("PC_115\r")
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) VolumeStatus() {
	msg := []byte("PC_/19\r")
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) VolumeInc() {
	log.Println("VolumeInc called")
	msg := []byte("PC_/11\r")
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) VolumeDec() {
	log.Println("VolumeDec called")
	msg := []byte("PC_/10\r")
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) VolumeSet(v int) {
	if v < 0 || v > 100 {
		log.Printf("SetVolume: volume must be between 0 and 100")
		return
	}
	log.Printf("SetVolume called with volume %d", v)
	msg := []byte("PC_01")
	msg = append(msg, 0x30+byte(v))
	msg = append(msg, 0x0d) // \r
	a.QueueWrite(msg)
}

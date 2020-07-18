package arcamctl

import (
	"fmt"
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
	Zone2Volume             int
	Zone1PoweredOn          bool
	Zone2PoweredOn          bool
	Zone1MuteOn             bool
	Zone2MuteOn             bool
	Zone1AudioSource        int
	Zone2AudioSource        int
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
	//log.Printf("handleStatusMessage: %s\n", msg)
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
	//log.Printf("checkStatusMessage: %s", msg)
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
	volume := int(val - 0x30)
	if zone == 0x31 {
		a.State.Zone1Volume = volume // TODO this needs to be concurrency safe
	}
	if zone == 0x32 {
		a.State.Zone2Volume = volume // TODO this needs to be concurrency safe
	}
}

func (a *ArcamAVRController) handlePowerStatus(msg string) {
	if !checkStatusMessage(msg, "AV_*", 7) {
		return
	}
	zone := byte(msg[5])
	val := byte(msg[6])
	state := false
	if val == 0x31 {
		state = true
	}
	if zone == 0x31 {
		//log.Printf("handlePowerStatus: setting zone1 to %s", state)
		a.State.Zone1PoweredOn = state
	} else if zone == 0x32 {
		//log.Printf("handlePowerStatus: setting zone2 to %s", state)
		a.State.Zone2PoweredOn = state
	} else {
		log.Printf("handlePowerStatus: invalid zone %q", zone)
	}
}

func (a *ArcamAVRController) handleMuteStatus(msg string) {
	if checkStatusMessage(msg, "AV_.", 7) == false {
		return
	}
	zone := byte(msg[5])
	val := byte(msg[6])
	state := false
	if val == 0x30 {
		state = true
	}

	if zone == 0x31 {
		a.State.Zone1MuteOn = state
	} else if zone == 0x32 {
		a.State.Zone2MuteOn = state
	} else {
		log.Printf("zone not handled: %s", zone)
	}
}

func (a *ArcamAVRController) handleSourceStatus(msg string) {
	if checkStatusMessage(msg, "AV_1", 7) == false {
		return
	}
	zone := byte(msg[5])
	val := byte(msg[6])
	// 0: 0x30 DVD
	// 1: 0x31 SAT
	// 2: 0x32 AV
	// 3: 0x33 PVR
	// 4: 0x34 VCR
	// 5: 0x35 CD
	// 6: 0x36 FM
	// 7: 0x37 AM
	// 8: 0x38 DVDA
	if zone == 0x31 {
		a.State.Zone1AudioSource = int(val) - 0x30
	}
	if zone == 0x32 {
		a.State.Zone2AudioSource = int(val) - 0x30
	}
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
	//log.Printf("QueueWrite: %s", msg)
	a.writeFifo.Enqueue(msg)
}

func (a *ArcamAVRController) QueueStatusPoller() {
	a.State.SerialWriterQueueLength = a.writeFifo.GetLen()
	time.Sleep(1 * time.Second)
}

func (a *ArcamAVRController) statusPoller() {
	for {
		//log.Printf("statusPoller: %s\n", a.State.Zone1PoweredOn)
		a.PowerStatus(1)
		if a.State.Zone1PoweredOn {
			// these return an 'R' code if amp is on standby,
			// so no point in polling if we aren't on.
			for z := 1; z < 3; z++ { // zone 1 and 2
				//log.Printf("statusPoller: polling inner status for zone %d", z)
				a.MuteStatus(z)
				a.VolumeStatus(z)
				a.PowerStatus(z)
				a.AudioSelectStatus(z)
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (a *ArcamAVRController) Power(m int, z int) {
	//log.Printf("Power: m:%d, z:%d\n", m, z)
	if m != 0 && m != 1 && m != 9 {
		log.Printf("Power: mode must be 0, 1 or 9")
		return
	}
	if z != 1 && z != 2 {
		log.Printf("Power: zone must be 1 or 2")
		return
	}
	msg := []byte(fmt.Sprintf("PC_*%d%d\r", z, m))
	//log.Printf("Power: msg: %s\n", msg)
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) PowerOn(z int) {
	a.Power(1, z)
}

func (a *ArcamAVRController) PowerOff(z int) {
	a.Power(0, z)
}

func (a *ArcamAVRController) PowerStatus(z int) {
	a.Power(9, z)
}

func (a *ArcamAVRController) MuteGeneric(m int, z int) {
	if m != 0 && m != 1 && m != 9 {
		log.Printf("MuteGeneric: mode must be 0, 1 or 9")
		return
	}
	if z != 1 && z != 2 {
		log.Printf("MuteGeneric: zone must be 1 or 2")
		return
	}
	msg := []byte(fmt.Sprintf("PC_.%d%d\r", z, m))
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) Mute(z int) {
	a.MuteGeneric(0, z)
}

func (a *ArcamAVRController) Unmute(z int) {
	a.MuteGeneric(1, z)
}

func (a *ArcamAVRController) MuteStatus(z int) {
	a.MuteGeneric(9, z)
}

func (a *ArcamAVRController) AudioSelect(s int, z int) {
	if s < 0 || s > 9 {
		log.Printf("AudioSelect: selection must be 0 thru 9")
		return
	}
	if z != 1 && z != 2 {
		log.Printf("AudioSelect: zone must be 1 or 2")
		return
	}
	msg := []byte(fmt.Sprintf("PC_1%d%d\r", z, s))
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) AudioSelectStatus(z int) {
	a.AudioSelect(9, z)
}

func (a *ArcamAVRController) AudioSelectSat(z int) {
	a.AudioSelect(1, z)
}

func (a *ArcamAVRController) AudioSelectPVR(z int) {
	a.AudioSelectAux(z)
}

func (a *ArcamAVRController) AudioSelectAux(z int) {
	a.AudioSelect(3, z)
}

func (a *ArcamAVRController) AudioSelectCD(z int) {
	a.AudioSelect(5, z)
}

func (a *ArcamAVRController) Volume(m int, z int) {
	if m != 0 && m != 1 && m != 9 {
		log.Printf("Volume: mode must be 0, 1 or 9")
		return
	}
	if z != 1 && z != 2 {
		log.Printf("Volume: zone must be 1 or 2")
		return
	}
	msg := []byte(fmt.Sprintf("PC_/%d%d\r", z, m))
	a.QueueWrite(msg)
}

func (a *ArcamAVRController) VolumeStatus(z int) {
	a.Volume(9, z)
}

func (a *ArcamAVRController) VolumeInc(z int) {
	a.Volume(1, z)
}

func (a *ArcamAVRController) VolumeDec(z int) {
	a.Volume(0, z)
}

func (a *ArcamAVRController) VolumeSet(v int, z int) {
	if v < 0 || v > 100 {
		log.Printf("SetVolume: volume must be between 0 and 100")
		return
	}
	if z != 1 && z != 2 {
		log.Printf("SetVolume: zone must be 1 or 2")
		return
	}
	log.Printf("SetVolume called with volume %d for zone %d", v, z)
	msg := []byte(fmt.Sprintf("PC_0%d", z))
	msg = append(msg, 0x30+byte(v))
	msg = append(msg, 0x0d) // \r
	a.QueueWrite(msg)
}

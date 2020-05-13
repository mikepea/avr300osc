package arcamctl

import (
	"github.com/tarm/serial"
	"log"
	"strings"
)

const SERIAL_DEV = "/dev/ttyUSB0"
const SERIAL_BAUD = 38400

var s *serial.Port

type ArcamAmpState struct {
	Zone1Volume int
	PoweredOn   bool
	MuteOn      bool
}

type ArcamAVRController struct {
	State      *ArcamAmpState
	serialPort *serial.Port
}

func NewArcamAVRController() (*ArcamAVRController, error) {
	log.Println("init: Opening port")
	c := &serial.Config{Name: SERIAL_DEV, Baud: SERIAL_BAUD}
	s, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}
	a := &ArcamAVRController{
		State:      &ArcamAmpState{},
		serialPort: s,
	}
	go a.reader()
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
		a.handleVolumeStatus(msg)
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

func (a *ArcamAVRController) handleVolumeSetStatus(msg string) {
	log.Printf("VolumeSetStatus: %s", msg)
	if msg[:5] != "AV_0P" {
		log.Printf("Invalid message: %s", msg)
	} else if len(msg) != 7 {
		log.Printf("Invalid message (wrong length): %s", msg)
		return
	}
	zone := byte(msg[5])
	vol := byte(msg[6])
	if zone != 0x31 {
		log.Printf("Zone2 not handled: %s", msg)
	}
	volume := int(vol - 0x30)
	log.Printf("Volume: %d", volume)
	a.State.Zone1Volume = volume // TODO this needs to be concurrency safe

}

func (a *ArcamAVRController) handleVolumeStatus(msg string) {
	log.Printf("VolumeStatus: %s", msg)
}

func (a *ArcamAVRController) handlePowerStatus(msg string) {
	log.Printf("PowerStatus: %s", msg)
}

func (a *ArcamAVRController) handleMuteStatus(msg string) {
	log.Printf("MuteStatus: %s", msg)
}

func (a *ArcamAVRController) handleSourceStatus(msg string) {
	log.Printf("SourceStatus: %s", msg)
}

func (a *ArcamAVRController) reader() {
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

func (a *ArcamAVRController) PowerOn() {
	log.Println("PowerOn called")
	_, err := a.serialPort.Write([]byte("PC_*11\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) PowerOff() {
	log.Println("PowerOff called")
	_, err := a.serialPort.Write([]byte("PC_*10\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) Mute() {
	log.Println("Mute called")
	_, err := a.serialPort.Write([]byte("PC_.10\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) Unmute() {
	log.Println("Unmute called")
	_, err := a.serialPort.Write([]byte("PC_.11\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) AudioSelectSat() {
	log.Println("AudioSelectSat called")
	_, err := a.serialPort.Write([]byte("PC_111\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) AudioSelectPVR() {
	a.AudioSelectAux()
}

func (a *ArcamAVRController) AudioSelectAux() {
	log.Println("AudioSelectAux called")
	_, err := a.serialPort.Write([]byte("PC_113\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) AudioSelectCD() {
	log.Println("AudioSelectCD called")
	_, err := a.serialPort.Write([]byte("PC_115\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) VolumeInc() {
	log.Println("VolumeInc called")
	_, err := a.serialPort.Write([]byte("PC_/11\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) VolumeDec() {
	log.Println("VolumeDec called")
	_, err := a.serialPort.Write([]byte("PC_/10\r"))
	if err != nil {
		log.Fatal(err)
	}
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
	_, err := a.serialPort.Write(msg)
	if err != nil {
		log.Fatal(err)
	}
}

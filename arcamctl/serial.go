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
	CurrentVolume int
	PoweredOn     bool
	MuteOn        bool
}

type ArcamAVRController struct {
	State *ArcamAmpState
	s     *serial.Port
}

func NewArcamAVRController() (*ArcamAVRController, error) {
	log.Println("init: Opening port")
	c := &serial.Config{Name: SERIAL_DEV, Baud: SERIAL_BAUD}
	s, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}
	go reader(s)
	return &ArcamAVRController{
		s: s,
	}, nil
}

func handleStatusMessages(msgs []string) {
	for _, msg := range msgs {
		HandleStatusMessage(msg)
	}
}

/*
*  AV_0 - volume set
*  AV_* - power
*  AV_. - mute
*  AV_/ - volume inc/dec/status
*  AV_1 - source select
 */
func HandleStatusMessage(msg string) {
	if len(msg) < 4 {
		log.Printf("message too short: %v\n", msg)
		return
	}

	family := msg[:4]
	switch family {
	case "AV_0":
		handleVolumeSetStatus(msg)
	case "AV_/":
		handleVolumeStatus(msg)
	case "AV_*":
		handlePowerStatus(msg)
	case "AV_.":
		handleMuteStatus(msg)
	case "AV_1":
		handleSourceStatus(msg)
	default:
		log.Printf("unhandled message family: %s\n", family)
	}
}

func handleVolumeSetStatus(msg string) {
	log.Printf("VolumeSetStatus: %s", msg)
}

func handleVolumeStatus(msg string) {
	log.Printf("VolumeStatus: %s", msg)
}

func handlePowerStatus(msg string) {
	log.Printf("PowerStatus: %s", msg)
}

func handleMuteStatus(msg string) {
	log.Printf("MuteStatus: %s", msg)
}

func handleSourceStatus(msg string) {
	log.Printf("SourceStatus: %s", msg)
}

func reader(s *serial.Port) {
	log.Println("Setting up port reader")
	var msgOverrun []byte
	for {

		buf := make([]byte, 128)
		n, err := s.Read(buf)
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

		handleStatusMessages(msgs)

	}
}

func (a *ArcamAVRController) PowerOn() {
	log.Println("PowerOn called")
	_, err := a.s.Write([]byte("PC_*11\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) PowerOff() {
	log.Println("PowerOff called")
	_, err := a.s.Write([]byte("PC_*10\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) Mute() {
	log.Println("Mute called")
	_, err := a.s.Write([]byte("PC_.10\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) Unmute() {
	log.Println("Unmute called")
	_, err := a.s.Write([]byte("PC_.11\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) AudioSelectSat() {
	log.Println("AudioSelectSat called")
	_, err := a.s.Write([]byte("PC_111\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) AudioSelectPVR() {
	a.AudioSelectAux()
}

func (a *ArcamAVRController) AudioSelectAux() {
	log.Println("AudioSelectAux called")
	_, err := a.s.Write([]byte("PC_113\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) AudioSelectCD() {
	log.Println("AudioSelectCD called")
	_, err := a.s.Write([]byte("PC_115\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) VolumeInc() {
	log.Println("VolumeInc called")
	_, err := a.s.Write([]byte("PC_/11\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *ArcamAVRController) VolumeDec() {
	log.Println("VolumeDec called")
	_, err := a.s.Write([]byte("PC_/10\r"))
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
	msg = append(msg, 0x31+byte(v))
	msg = append(msg, 0x0d) // \r
	_, err := a.s.Write(msg)
	if err != nil {
		log.Fatal(err)
	}
}

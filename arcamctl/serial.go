package arcamctl

import (
	"github.com/tarm/serial"
	"log"
)

const SERIAL_DEV = "/dev/ttyUSB0"
const SERIAL_BAUD = 38400

var s *serial.Port

func init() {
	log.Println("init: Opening port")
	c := &serial.Config{Name: SERIAL_DEV, Baud: SERIAL_BAUD}
	var err error
	s, err = serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}
}

func PowerOn() {
	log.Println("PowerOn called")
	_, err := s.Write([]byte("PC_*11\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func PowerOff() {
	log.Println("PowerOff called")
	_, err := s.Write([]byte("PC_*10\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func Mute() {
	log.Println("Mute called")
	_, err := s.Write([]byte("PC_.10\r"))
	if err != nil {
		log.Fatal(err)
	}
}

func Unmute() {
	log.Println("Unmute called")
	_, err := s.Write([]byte("PC_.11\r"))
	if err != nil {
		log.Fatal(err)
	}
}

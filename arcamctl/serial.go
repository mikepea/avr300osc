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

	go reader(s)
}

func reader(s *serial.Port) {
	log.Println("Setting up port reader")
	for {
		buf := make([]byte, 128)
		n, err := s.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("%q", buf[:n])
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

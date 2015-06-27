package main

import (
	"github.com/madhurjain/lirc"
	"github.com/stianeikeland/go-rpio"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	// Triggered State
	TRIGGERED = iota
	ARMED
	DISARMED
)

type burglarm struct {
	buzzerPin rpio.Pin // PIN 15, GPIO 22
	pirPin    rpio.Pin // PIN 16, GPIO 23
	action    chan uint8
	stop      chan bool
}

func (b *burglarm) pollPir() {
	b.buzzerPin.High()
	time.Sleep(time.Second)
	b.buzzerPin.Low()
	// Keep reading PIR pin for trigger
	for {
		if b.pirPin.Read() == rpio.High {
			b.action <- TRIGGERED
			return
		}
	}
}

func (b *burglarm) start() {
	for {
		select {
		case action := <-b.action:
			switch action {
			case ARMED:
				// Start monitoring after 5 seconds the device is turned on
				time.AfterFunc(5*time.Second, func() { b.pollPir() })
			case DISARMED:
				b.buzzerPin.Low()
			case TRIGGERED:
				// Set alarm pin high
				b.buzzerPin.High()
			}

		case <-b.stop:
			b.buzzerPin.Low()
			log.Println("stopping service")
			b.stop <- true
			return
		}
	}
}

func (b *burglarm) terminate() {
	b.stop <- true
	<-b.stop // block until previous stop is processed
}

func irevent(ev lirc.LircEvent) {
	log.Println("remote event")
	log.Println(ev)
}

func main() {

	// Open memory range for GPIO access in /dev/mem
	err := rpio.Open()
	if err != nil {
		log.Println(err)
	}
	defer rpio.Close()

	b := &burglarm{
		action: make(chan uint8),
		stop:   make(chan bool),
	}
	// Init GPIO Pins
	b.buzzerPin = rpio.Pin(22)
	b.buzzerPin.Mode(rpio.Output)

	b.pirPin = rpio.Pin(23)
	b.pirPin.Mode(rpio.Input)

	// Init IR
	ir, err := lirc.Init("")
	if err != nil {
		log.Println("LIRC Error", err)
	}
	ir.Handle("", "", irevent)
	//ir.Run()

	// Start Service
	log.Println("Starting Service")
	go burglarm.start()
	burglarm.action <- ARMED // Arm alarm on start

	// Stop Service
	// Handle SIGINT and SIGTERM
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.Println(<-ch) // blocks main

	burglarm.terminate()

	// Disarm on key press of remote
	// Rearm on key press of remote

}

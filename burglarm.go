package main

import (
	"github.com/chbmuc/lirc"
	"github.com/stianeikeland/go-rpio"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	// Burglarm States
	TRIGGERED = iota
	ARMED
	DISARMED
)

type Burglarm struct {
	buzzerPin rpio.Pin // PIN 15, GPIO 22
	pirPin    rpio.Pin // PIN 16, GPIO 23
	state     uint8
	action    chan uint8
	stop      chan bool
}

func (b *Burglarm) pollPir() {
	// Keep reading PIR pin for trigger
	for {
		if b.pirPin.Read() == rpio.High {
			b.action <- TRIGGERED
			return
		}
	}
}

func (b *Burglarm) beep() {
	b.buzzerPin.High()
	time.Sleep(time.Millisecond * 500)
	b.buzzerPin.Low()
}

func (b *Burglarm) start() {
	for {
		select {
		case action := <-b.action:
			switch action {
			case ARMED:
				log.Println("Armed")
				// Start monitoring after 5 seconds the device is turned on
				time.AfterFunc(5*time.Second, func() { b.pollPir() })
			case DISARMED:
				log.Println("Disarmed")
				b.buzzerPin.Low()
			case TRIGGERED:
				log.Println("Triggered")
				// Set alarm pin high
				b.buzzerPin.High()
			}
			b.state = action
		case <-b.stop:
			b.buzzerPin.Low()
			log.Println("stopping service")
			b.stop <- true
			return
		}
	}
}

func (b *Burglarm) terminate() {
	b.stop <- true
	<-b.stop // block until previous stop is processed
}

func (b *Burglarm) remoteKey(event lirc.Event) {

	// Long key press triggers event twice
	// Ignore long key presses
	if event.Repeat > 0 {
		return
	}
	log.Println(event.Button)
	b.beep()
	if b.state == DISARMED {
		// Rearm on key press of remote
		log.Println("ARM")
		b.action <- ARMED
	} else {
		// Disarm on key press of remote
		log.Println("DISARM")
		b.action <- DISARMED
	}
}

func main() {

	// Open memory range for GPIO access in /dev/mem
	err := rpio.Open()
	if err != nil {
		log.Println(err)
	}
	defer rpio.Close()

	burglarm := &Burglarm{
		action: make(chan uint8),
		stop:   make(chan bool),
	}
	// Init GPIO Pins
	burglarm.buzzerPin = rpio.Pin(22)
	burglarm.buzzerPin.Mode(rpio.Output)

	burglarm.pirPin = rpio.Pin(23)
	burglarm.pirPin.Mode(rpio.Input)

	// Init IR
	ir, err := lirc.Init("/var/run/lirc/lircd")
	if err != nil {
		log.Println("LIRC Error", err)
	}
	ir.Handle("", "KEY_POWER", burglarm.remoteKey)
	go ir.Run()

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

}

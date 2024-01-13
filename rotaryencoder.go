//go:build ignore

/* Decording a rotary encoder, but uses periph.io.
TODO: would be to use go-rpio but we don't have a
rotary encoder in this project anyway */

package main

import (
	"log"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

type State struct {
	Pin   gpio.PinIO
	Level gpio.Level
}

type RotaryEncoder struct {
	aPin  gpio.PinIO
	bPin  gpio.PinIO
	state [4]State

	cw  [4]State
	ccw [4]State
}

func (t *RotaryEncoder) Init() {
	t.cw = [4]State{
		{
			Pin:   t.aPin,
			Level: gpio.High,
		},
		{
			Pin:   t.bPin,
			Level: gpio.High,
		},
		{
			Pin:   t.aPin,
			Level: gpio.Low,
		},
		{
			Pin:   t.bPin,
			Level: gpio.Low,
		},
	}

	t.ccw = [4]State{
		{
			Pin:   t.bPin,
			Level: gpio.High,
		},
		{
			Pin:   t.aPin,
			Level: gpio.High,
		},
		{
			Pin:   t.bPin,
			Level: gpio.Low,
		},
		{
			Pin:   t.aPin,
			Level: gpio.Low,
		},
	}
}

func NewRotaryEncoder(pin1 string, pin2 string) *RotaryEncoder {
	aPin := gpioreg.ByName(pin1)
	bPin := gpioreg.ByName(pin2)
	if err := aPin.In(gpio.PullUp, gpio.BothEdges); err != nil {
		log.Fatal(err)
	}
	if err := bPin.In(gpio.PullUp, gpio.BothEdges); err != nil {
		log.Fatal(err)
	}
	t := &RotaryEncoder{
		aPin: aPin,
		bPin: bPin,
	}
	t.Init()
	triggerChan := make(chan gpio.PinIO)
	go func() {
		for {
			pin := <-triggerChan
			state := State{Level: pin.Read(), Pin: pin}
			if state == t.state[3] {
				continue
			}
			t.state = [4]State{
				t.state[1],
				t.state[2],
				t.state[3],
				state,
			}
			if (t.state[0] == t.cw[0] && t.state[1] == t.cw[1] && t.state[2] == t.cw[2] && t.state[3] == t.cw[3]) || (t.state[0] == t.cw[1] && t.state[1] == t.cw[2] && t.state[2] == t.cw[3] && t.state[3] == t.cw[0]) || (t.state[0] == t.cw[2] && t.state[1] == t.cw[3] && t.state[2] == t.cw[0] && t.state[3] == t.cw[1]) || (t.state[0] == t.cw[3] && t.state[1] == t.cw[0] && t.state[2] == t.cw[1] && t.state[3] == t.cw[2]) {
				t.state = [4]State{}
				log.Printf("[%d] [Encoder] CW", time.Now().UnixNano())
			} else if (t.state[0] == t.ccw[0] && t.state[1] == t.ccw[1] && t.state[2] == t.ccw[2] && t.state[3] == t.ccw[3]) || (t.state[0] == t.ccw[1] && t.state[1] == t.ccw[2] && t.state[2] == t.ccw[3] && t.state[3] == t.ccw[0]) || (t.state[0] == t.ccw[2] && t.state[1] == t.ccw[3] && t.state[2] == t.ccw[0] && t.state[3] == t.ccw[1]) || (t.state[0] == t.ccw[3] && t.state[1] == t.ccw[0] && t.state[2] == t.ccw[1] && t.state[3] == t.ccw[2]) {
				t.state = [4]State{}
				log.Printf("[%d] [Encoder] CCW", time.Now().UnixNano())
			}
		}
	}()
	for _, pin := range []gpio.PinIO{aPin, bPin} {
		go func(pin gpio.PinIO) {
			for {
				if pin.WaitForEdge(-1) {
					triggerChan <- pin
				}
			}
		}(pin)
	}
	return t
}

type Rotary8421Encoder struct {
	State int
}

func New8421Encoder(pin1 string, pin2 string, pin3 string, pin4 string) *Rotary8421Encoder {
	enc := &Rotary8421Encoder{}
	s1 := gpioreg.ByName(pin1)
	s2 := gpioreg.ByName(pin2)
	s3 := gpioreg.ByName(pin3)
	s4 := gpioreg.ByName(pin4)
	triggerChan := make(chan int)
	debounceDuration := 1250 * time.Millisecond
	timeout := time.NewTimer(debounceDuration)
	go func() {
		for {
			<-timeout.C
			bools := []gpio.Level{s1.Read(), s2.Read(), s3.Read(), s4.Read()}
			var number int
			for i, b := range bools {
				if b {
					number |= 1 << (len(bools) - 1 - i)
				}
			}
			enc.State = number
			log.Println("[8421] State: ", number)
		}
	}()
	go func() {
		for {
			triggerPin := <-triggerChan
			timeout.Stop()
			timeout.Reset(debounceDuration)
			log.Printf("[8421] Triggered: %d", triggerPin)
		}
	}()
	for _, key := range []gpio.PinIO{s1, s2, s3, s4} {
		if err := key.In(gpio.PullDown, gpio.RisingEdge); err != nil {
			log.Fatal(err)
		}
		go func(key gpio.PinIO) {
			for {
				if key.WaitForEdge(-1) {
					triggerChan <- key.Number()
				}
			}
		}(key)
	}
	return enc
}

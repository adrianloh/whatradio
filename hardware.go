package main

import (
	"os/exec"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

const (
	BTN_SHIFT       = 5
	BTN_MUTED       = 6
	BTN_PLAY_RANDOM = 16
	BTN_PLAY_FAV    = 24
)

var (
	CMD_MUTE   = []byte("set Master 0%\n")
	CMD_MAXVOL = []byte("set Master 100%\n")
)

var (
	SHIFT_BUTTON = rpio.Pin(BTN_SHIFT)
)

func setup_shift_button() {
	SHIFT_BUTTON.Input()
	SHIFT_BUTTON.PullUp()
}

func setup_mute_button() {
	muted := false
	muteChan := make(chan bool)
	amixerCmd := exec.Command("amixer", "-s")
	amixerInput, _ := amixerCmd.StdinPipe()
	go amixerCmd.Start()
	go amixerCmd.Wait()
	amixerInput.Write(CMD_MAXVOL)
	go on_press(BTN_MUTED, muteChan)
	for {
		<-muteChan
		if muted {
			amixerInput.Write(CMD_MAXVOL)
			muted = false
		} else {
			amixerInput.Write(CMD_MUTE)
			muted = true
		}
	}
}

func on_press(pinNumber int, c chan bool) {
	pin := rpio.Pin(pinNumber)
	pin.Input()
	pin.PullUp()
	pin.Detect(rpio.FallEdge)
	defer pin.Detect(rpio.NoEdge)
	last_call := time.Now()
	for {
		timeNow := time.Now()
		if pin.EdgeDetected() && timeNow.Sub(last_call) > time.Millisecond*1000 {
			last_call = timeNow
			//log.Printf("Pin %d [PRESS] at %d\n", pinNumber, timeNow.UnixNano())
			c <- true
		}
		time.Sleep(time.Millisecond * 50)
	}
}

func on_press_or_hold(pinNumber int, press chan bool, hold chan bool) {
	pin := rpio.Pin(pinNumber)
	pin.Input()
	pin.PullUp()
	pin.Detect(rpio.FallEdge)
	defer pin.Detect(rpio.NoEdge)
	last_call := time.Now()
	for {
		timeNow := time.Now()
		if pin.EdgeDetected() && timeNow.Sub(last_call) > time.Millisecond*1000 {
			last_call = timeNow
			go func() {
				time.Sleep(time.Millisecond * 500)
				if pin.Read() == rpio.Low {
					//log.Printf("Pin %d [HOLD] at %d\n", pinNumber, timeNow.UnixNano())
					hold <- true
				} else {
					//log.Printf("Pin %d [PRESS] at %d\n", pinNumber, timeNow.UnixNano())
					press <- true
				}
			}()
		}
		time.Sleep(time.Millisecond * 50)
	}
}

// func led_test() {
// 	leds := []rpio.Pin{rpio.Pin(12), rpio.Pin(16), rpio.Pin(20), rpio.Pin(13), rpio.Pin(19), rpio.Pin(26)}
// 	for _, led := range leds {
// 		led.Output()
// 	}

// 	for _, led := range leds {
// 		go blink(led)
// 	}

// 	time.Sleep(time.Second * 15)
// }

// func _blink(led rpio.Pin) {
// 	for x := 0; x < 20; x++ {
// 		led.Toggle()
// 		time.Sleep(time.Second / 5)
// 	}
// }

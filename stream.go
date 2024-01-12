package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

type Buff struct {
	FirstChunk      bool
	Sink            *AudioSink
	PreviousStation *StationStream
	Failtimer       *time.Timer
	DataStarted     chan bool
	LastRead        time.Time
}

func (buff *Buff) Write(b []byte) (n int, err error) {
	if buff.FirstChunk {
		buff.Failtimer.Stop()
		buff.PreviousStation.Stop()
		buff.FirstChunk = false
		buff.DataStarted <- true
	}
	buff.LastRead = time.Now()
	buff.Sink.Write(b)
	return len(b), nil
}

type StationStream struct {
	Station
	Buff          *Buff
	Process       *exec.Cmd
	FFMessages    io.ReadCloser
	CancelMonitor context.CancelFunc
	Started       bool
}

func (stream *StationStream) Monitor(playRandom chan bool, display *Display) {

	ctx, cancel := context.WithCancel(context.Background())

	stream.CancelMonitor = cancel

	checkDataStream := time.NewTicker(5 * time.Second)
	streamDataStopped := make(chan bool)

	max_silence := 60 * time.Second

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-checkDataStream.C:
				if time.Since(stream.Buff.LastRead) > 15*time.Second {
					streamDataStopped <- true
				}
			}
		}
	}()

	scanner := bufio.NewScanner(stream.FFMessages)
	scanner.Split(bufio.ScanWords)

	silentTimeout := time.NewTimer(max_silence)
	silentTimeout.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				scanner.Scan()
				txt := scanner.Text()
				if strings.Contains(txt, "silence_start") {
					fmt.Println("[STREAM] Station has gone quiet")
					display.ShowStatus <- HUH
					silentTimeout.Reset(max_silence)
				}
				if strings.Contains(txt, "silence_end") {
					display.ShowStatus <- PLAYING
					fmt.Println("[STREAM] Resumed")
					silentTimeout.Stop()
				}
			}
		}
	}()

monitorLoop:
	for {
		select {
		case <-ctx.Done():
			// This only happens to this main loop when the next station calls `Stop()`
			return
		case <-streamDataStopped:
			fmt.Printf("[STREAM] No data received for %d seconds\n", int(time.Since(stream.Buff.LastRead).Seconds()))
			break monitorLoop
		case <-silentTimeout.C:
			fmt.Println("[STREAM] Too much quiet, moving on...")
			break monitorLoop
		}
	}

	cancel()
	playRandom <- true

}

func (stream *StationStream) Stop() {
	if stream.CancelMonitor != nil {
		stream.CancelMonitor()
	}
	stream.Process.Process.Kill()
}

func NewStationStream(station Station, sink *AudioSink, prevStation *StationStream, result chan StationStream) {
	fmt.Printf("[ GET ]: %s\n", station.Name)
	buff := &Buff{
		true,
		sink,
		prevStation,
		time.NewTimer(30 * time.Second), // How long to wait for this station to start before aborting
		make(chan bool),
		time.Now(),
	}
	ffmpegCmd := exec.Command("ffmpeg", "-hide_banner",
		"-i", station.URL,
		"-f", "wav",
		"-af", "loudnorm=I=-14:LRA=7:TP=-2",
		"-af", "silencedetect=noise=-30dB:d=20", // Detect silence -30dB, trigger `silence_detected` after 20 seconds
		"-ar", "44100",
		"-ac", "2",
		"-")
	ffmpegOut, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	ffmpegErr, _ := ffmpegCmd.StderrPipe()
	if err := ffmpegCmd.Start(); err != nil {
		panic(err)
	}
	go ffmpegCmd.Wait()
	go func() {
		_, err := io.Copy(buff, ffmpegOut)
		if err != nil {
			// Happens when we kill ffmpeg deliberately, or, when something craps out with the stream
			fmt.Printf("[STREAM] ended: %s\n", station.Name)
		}
	}()
	stationProcess := StationStream{
		station,
		buff,
		ffmpegCmd,
		ffmpegErr,
		nil,
		false}
	select {
	case <-buff.DataStarted:
		fmt.Printf("[STREAM] started: %s\n", station.Name)
		stationProcess.Started = true
		result <- stationProcess
	case <-buff.Failtimer.C:
		ffmpegCmd.Process.Kill()
		result <- stationProcess
	}
}

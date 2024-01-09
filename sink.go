package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type AudioSink struct {
	Analyzer     io.Writer
	Player       *exec.Cmd
	PlayerIn     io.Writer
	Record       bool
	RecordBuffer io.Writer
	TempBuffer   bytes.Buffer
	LastRead     time.Time
	CurrDB       float64
}

func (sink *AudioSink) Init() {
	sink.newPlayer()
	analyzer := exec.Command("ffmpeg",
		"-f", "s16le",
		"-ar", "44100",
		"-ac", "2",
		"-i", "-",
		"-af", "asetnsamples=44100,astats=metadata=1:reset=1,ametadata=print:key=lavfi.astats.Overall.RMS_level",
		"-f", "null",
		"-")
	sink.Analyzer, _ = analyzer.StdinPipe()
	// analyzer_out, _ := analyzer.StderrPipe()
	// go func() {
	// 	re := regexp.MustCompile(`^lavfi.astats.Overall.RMS_level=(-?\d{1,2}\.\d+$)`)
	// 	scanner := bufio.NewScanner(analyzer_out)
	// 	scanner.Split(bufio.ScanWords)
	// 	for {
	// 		scanner.Scan()
	// 		txt := scanner.Text()
	// 		m := re.FindAllStringSubmatch(txt, -1)
	// 		if len(m) > 0 {
	// 			v, err := strconv.ParseFloat(m[0][1], 64)
	// 			if err == nil {
	// 				sink.CurrDB = v
	// 			}
	// 		}
	// 	}
	// }()
	//analyzer.Start()
}

func (sink *AudioSink) newPlayer() {
	if sink.Player != nil {
		sink.Player.Process.Kill()
	}
	aplayCmd := exec.Command("aplay", "-")
	stdin, err := aplayCmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	if err := aplayCmd.Start(); err != nil {
		panic(err)
	}
	go aplayCmd.Wait()
	sink.Player = aplayCmd
	sink.PlayerIn = stdin
}

func (sink *AudioSink) Write(b []byte) (n int, err error) {
	sink.PlayerIn.Write(b)
	//sink.Analyzer.Write(b)
	sink.LastRead = time.Now()
	if sink.Record {
		sink.RecordBuffer.Write(b)
	}
	return len(b), nil
}

func (sink *AudioSink) Close() error {
	return nil
}

func (sink *AudioSink) RecordSample() (string, error) {

	if sink.Record {
		return "", errors.New("Already recording")
	}

	recordPath := filepath.Join(HOME, "clip-recording.mp3")

	mp3Encoder := exec.Command("ffmpeg",
		"-f", "s16le",
		"-ar", "44100",
		"-ac", "2",
		"-i", "-",
		"-y",
		recordPath)

	mp3Encoder_input, _ := mp3Encoder.StdinPipe()

	buff := bytes.NewBuffer([]byte{})

	sink.RecordBuffer = buff

	mp3Encoder.Start()

	sink.Record = true

	time.Sleep(5 * time.Second)

	sink.Record = false

	io.Copy(mp3Encoder_input, buff)

	mp3Encoder_input.Close()

	mp3Encoder.Wait()

	fifo, err := os.Stat(recordPath)
	if err != nil {
		return "", errors.New("Recording failed: " + err.Error())
	}
	if fifo.Size() == 0 {
		return "", errors.New("Recording failed: empty file")
	}

	return recordPath, nil

}

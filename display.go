package main

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/rubiojr/go-pirateaudio/display"
	"github.com/skip2/go-qrcode"
)

var STATUS_IMAGES_PATH = "gifs"

const (
	PERMANENT = iota
	STATIC
	SPLASH
	SEARCH
	PLAYING
	PLAYFAV
	ADDFAV
	ERROR
	IDENTIFY
	OKAY
	HUH
)

type StatusConfig struct {
	String       string
	RefreshRate  int
	RestoreState int
}

type QR struct {
	String    string
	Temporary int
}

var DISPLAY_CONFIGS = map[int]StatusConfig{

	// `PERMANENT` means the animation will play forever until a new animation overrides it

	SPLASH:  StatusConfig{`splash`, 100, PERMANENT},
	PLAYING: StatusConfig{`play`, 100, PERMANENT},

	SEARCH:   StatusConfig{`search`, 100, PERMANENT},
	PLAYFAV:  StatusConfig{`playfav`, 100, PERMANENT},
	IDENTIFY: StatusConfig{`identify`, 100, PERMANENT},
	ADDFAV:   StatusConfig{`addfav`, 100, PLAYING},

	ERROR: StatusConfig{`error`, 100, PLAYING},
	OKAY:  StatusConfig{`okay`, 100, PLAYING},
	HUH:   StatusConfig{`huh`, 100, PLAYING},
}

type Display struct {
	dsp           *display.Display
	imageBuffer   []*InfiniteReader
	cancel        context.CancelFunc
	last_set      map[string]int
	last_frame    map[string]int
	renderChan    chan int
	currentStatus int
	ShowStatus    chan int
	ShowQR        chan QR
}

func NewDisplay() (*Display, error) {
	d := &Display{}
	err := d.Init()
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Display) Init() error {
	dsp, err := display.Init()
	if err != nil {
		return err
	}
	d.dsp = dsp
	d.dsp.FillScreen(color.RGBA{R: 0, G: 0, B: 0, A: 0})
	last_set, last_frame, err := processDirectory(STATUS_IMAGES_PATH)
	if err != nil {
		return err
	}
	d.last_set = last_set
	d.last_frame = last_frame
	d.ShowStatus = make(chan int)
	d.ShowQR = make(chan QR)
	go func() {
		for {
			select {
			case status := <-d.ShowStatus:
				if status == d.currentStatus {
					continue
				}
				d.showStatus(status)
			case qr := <-d.ShowQR:
				d.showQR(qr.String, qr.Temporary)
			}
		}
	}()
	return nil
}

func (d *Display) showQR(str string, temporary int) error {
	png, err := qrcode.Encode(str, qrcode.Medium, 240)
	if err != nil {
		return err
	}
	pngReader := bytes.NewReader(png)
	imageInfiniteReader, _ := NewInfiniteReader(pngReader)
	if d.cancel != nil {
		d.cancel()
	}
	d.cancel = nil
	if temporary > 0 {
		go d.restorePreviousStatusAfter(temporary, d.currentStatus)
	}
	d.currentStatus = STATIC
	d.imageBuffer = []*InfiniteReader{imageInfiniteReader}
	go d.displayStatic()
	return nil
}

func (d *Display) showStatus(status int) {
	config := DISPLAY_CONFIGS[status]
	file_prefix := config.String
	err := d.checkImages(file_prefix)
	if err != nil {
		log.Printf("[DISPLAY: %s] Error: %v", file_prefix, err)
		return
	}
	err = d.loadImages(config.String)
	if err != nil {
		log.Printf("[DISPLAY: %s] Error: %v", config.String, err)
		return
	}
	// If we get here, the previous animation loop has been canceled and the new buffer has been loaded
	if config.RestoreState != PERMANENT {
		go d.restorePreviousStatusAfter(5, config.RestoreState)
	}
	d.currentStatus = status
	go d.playAnimation(config.RefreshRate)
}

func (d *Display) checkImages(prefix string) error {
	_, ok := d.last_set[prefix]
	if !ok {
		return fmt.Errorf(fmt.Sprintf("no images for `%s`", prefix))
	}
	return nil
}

func (d *Display) loadImages(prefix string) error {
	max := d.last_set[prefix]
	set := 0
	if max > 0 {
		set = rand.Intn(max + 1)
	}
	prefix_with_set := prefix + strconv.Itoa(set)
	last_frame, ok := d.last_frame[prefix_with_set]
	if !ok || last_frame == 0 {
		return fmt.Errorf("[DISPLAY] No images loaded `%s` set: %d last: %d", prefix, max, last_frame)
	}
	images := []*InfiniteReader{}
	// frames := getIntegersAtRegularIntervals(last_frame, 12)
	for i := 0; i <= last_frame; i++ {
		// for _, i := range frames {
		fp := filepath.Join(STATUS_IMAGES_PATH, fmt.Sprintf("%s_%03d.gif", prefix_with_set, i))
		img, err := os.Open(fp)
		if err != nil {
			return fmt.Errorf("[DISPLAY] Error opening file `%v` set: %d last: %d", err, max, last_frame)
		}
		InfiniteReader, _ := NewInfiniteReader(img)
		images = append(images, InfiniteReader)
	}
	if len(images) == 0 {
		return fmt.Errorf("[DISPLAY] No images loaded `%s` set: %d last: %d", prefix, max, last_frame)
	}
	// You must cancel the previous animation before loading a new one
	if d.cancel != nil {
		d.cancel()
	}
	d.imageBuffer = images
	return nil
}

func (d *Display) displayStatic() {
	img := d.imageBuffer[0]
	d.dsp.FillScreen(color.RGBA{R: 0, G: 0, B: 0, A: 0})
	d.dsp.DrawImage(img)
}

func (d *Display) playAnimation(refreshRate int) {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	ticker := time.NewTicker(time.Duration(refreshRate) * time.Millisecond)
	i := 0
	for {
		select {
		case <-ticker.C:
			img := d.imageBuffer[i]
			d.dsp.DrawImage(img)
			i = (i + 1) % len(d.imageBuffer)
		case <-ctx.Done():
			return
		}
	}
}

func (d *Display) restorePreviousStatusAfter(wait int, prevState int) {
	time.Sleep(time.Duration(wait) * time.Second)
	d.ShowStatus <- prevState
}

func getIntegersAtRegularIntervals(x, y int) []int {
	if y <= 1 {
		// If y is 1 or less, return only the start of the range
		return []int{0}
	}
	if y == 2 {
		// If y is 2, return the start and end of the range
		return []int{0, x}
	}

	interval := x / (y - 1)
	var numbers []int

	for i := 0; i < y; i++ {
		numbers = append(numbers, i*interval)
		if i*interval >= x {
			break
		}
	}

	// Correct the last number to be exactly x if it's not
	if numbers[len(numbers)-1] != x {
		numbers[len(numbers)-1] = x
	}

	return numbers
}

type InfiniteReader struct {
	data []byte
	pos  int
}

func NewInfiniteReader(r io.Reader) (*InfiniteReader, error) {
	// Read all data into memory
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &InfiniteReader{data: data}, nil
}

func (ir *InfiniteReader) Read(p []byte) (n int, err error) {
	if len(ir.data) == 0 {
		return 0, io.EOF // Handle empty data case
	}
	for i := range p {
		p[i] = ir.data[ir.pos]
		ir.pos++
		if ir.pos >= len(ir.data) {
			ir.pos = 0 // Reset to the beginning
			return i + 1, io.EOF
		}
	}
	return len(p), nil
}

// parseFilename extracts prefix, first counter, and second counter from the filename
func parseFilename(filename string) (string, int, int, error) {
	re := regexp.MustCompile(`^([a-zA-Z]+)(\d+)_(\d+)\.gif$`)
	matches := re.FindStringSubmatch(filename)

	if len(matches) != 4 {
		return "", 0, 0, fmt.Errorf("filename does not match pattern")
	}

	firstCounter, err := strconv.Atoi(matches[2])
	if err != nil {
		return "", 0, 0, err
	}

	secondCounter, err := strconv.Atoi(matches[3])
	if err != nil {
		return "", 0, 0, err
	}

	return matches[1], firstCounter, secondCounter, nil
}

// processDirectory processes the files in the directory and returns two lookup tables
func processDirectory(dirPath string) (map[string]int, map[string]int, error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, nil, err
	}

	lastSet := make(map[string]int)
	lastFrame := make(map[string]int)
	for _, file := range files {
		if file.IsDir() {
			continue // Skip directories
		}

		prefix, firstCounter, secondCounter, err := parseFilename(file.Name())
		if err != nil {
			fmt.Println("Error parsing filename:", err)
			continue
		}

		// Update lastSet map
		if current, exists := lastSet[prefix]; !exists || current < firstCounter {
			lastSet[prefix] = firstCounter
		}

		// Update lastFrame map
		fullPrefix := fmt.Sprintf("%s%d", prefix, firstCounter)
		if current, exists := lastFrame[fullPrefix]; !exists || current < secondCounter {
			lastFrame[fullPrefix] = secondCounter
		}
	}

	return lastSet, lastFrame, nil
}

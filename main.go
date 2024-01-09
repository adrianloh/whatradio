package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

var HOME = getExecutableDirectory()

type StationProcess struct {
	Station
	Process *exec.Cmd
	Started bool
}

type Status struct {
	Name string
	OK   bool
}

func main() {

	// Required by fs.go
	FAVORITES_FILE = filepath.Join(HOME, FAVORITES_FILE)

	// Required by display.go
	STATUS_IMAGES_PATH = filepath.Join(HOME, STATUS_IMAGES_PATH)
	if _, err := os.Stat(STATUS_IMAGES_PATH); os.IsNotExist(err) {
		log.Printf("[DISPLAY] No status animations path: %s", STATUS_IMAGES_PATH)
		os.Exit(1)
	}

	// GPIO
	if err := rpio.Open(); err != nil {
		log.Printf("[GPIO] Failed to open GPIO: %s", err)
		os.Exit(1)
	}
	defer rpio.Close()

	// Init display ST7789
	display, err := NewDisplay()
	if err != nil {
		log.Printf("[DISPLAY] Failed to init display: %s", err)
		os.Exit(1)
	}

	// Check if ffmpeg is installed
	ffmpegCmd := exec.Command("ffmpeg")
	err = ffmpegCmd.Start()
	if err != nil {
		log.Printf("[FFMPEG] Failed to start: %s", err)
		os.Exit(1)
	}

	var spotifyClient *SpotifyClient

	// To enable Spotify, create an empty file called `spotify_token.txt`
	// in the same directory as the binary.
	SPOTIFY_TOKEN_FILE = filepath.Join(HOME, SPOTIFY_TOKEN_FILE)
	_, err = os.ReadFile(SPOTIFY_TOKEN_FILE)
	if err == nil {
		log.Printf("[SPOTIFY] Token file found, enabling Spotify")
		spotifyClient, err = NewSpotifyClient(display)
		if err != nil {
			log.Printf("[SPOTIFY] Failed to init Spotify client: %s", err)
			os.Exit(1)
		}
	}

	display.ShowStatus(SPLASH)

	// `X` button
	playRandom := make(chan bool)
	identifySong := make(chan bool)
	go on_press_or_hold(BTN_PLAY_RANDOM, playRandom, identifySong)

	// `Y` button
	playFav := make(chan bool)
	saveFav := make(chan bool)
	go on_press_or_hold(BTN_PLAY_FAV, playFav, saveFav)

	// Channel to play station, sent from #playRandom and #playFav
	playStation := make(chan Station)

	// Channel receiving the result of #play_station
	nextStationResult := make(chan StationProcess)

	identifySongResult := make(chan Track)

	// The current station, set after receiving the result on nextStationResult
	var currentStation = &StationProcess{
		Process: ffmpegCmd,
	}

	audioSink := new(AudioSink)
	audioSink.Init()

	favorite_stations := getFavoriteStations() // this is *never* empty

	// Used to debounce button presses
	isPlaying := false

	// Volume control
	go setup_mute_button()

	// Thread that handles button presses
	go func() {
		for {
			select {
			case <-playFav:
				if isPlaying {
					log.Printf("[BUSY]")
					continue
				}
				isPlaying = true
				display.ShowStatus(PLAYFAV)
				otherStations := []Station{}
				for _, station := range favorite_stations {
					if station.UUID != currentStation.UUID {
						otherStations = append(otherStations, station)
					}
				}
				if len(otherStations) > 1 {
					playStation <- PickOne(otherStations)
				} else {
					playStation <- otherStations[0]
				}
			case <-playRandom:
				if isPlaying {
					log.Printf("[BUSY]")
					continue
				}
				isPlaying = true
				display.ShowStatus(SEARCH)
				log.Printf("[STATIONS] Getting random station")
				station, err := get_random_station(currentStation.Station)
				if err != nil {
					log.Printf("Failed to fetch new station: %s", err)
					isPlaying = false
					display.ShowStatus(ERROR)
					continue
				}
				playStation <- station
			case <-saveFav:
				display.ShowStatus(ADDFAV)
				err := add_favorite_station(currentStation.Station, favorite_stations)
				if err != nil {
					log.Printf("Failed to save favorite station: %s", err)
					continue
				}
				favorite_stations = append(favorite_stations, currentStation.Station)
				log.Printf("[FAVORITES] [%d] Added: %s", len(favorite_stations), currentStation.Station.Name)
			case <-identifySong:
				go RecordAndIdentifySong(audioSink, identifySongResult)
				display.ShowStatus(IDENTIFY)
			}
		}
	}()

	go func() {
		for {
			select {
			case station := <-playStation:
				go play_station(station, audioSink, currentStation.Process.Process, nextStationResult)
			case station := <-nextStationResult:
				if station.Started {
					display.ShowStatus(PLAYING)
					log.Printf("SET: %s", station.Name)
					currentStation = &station
				} else {
					log.Printf("[TIMEOUT] Station did not start")
					display.ShowStatus(ERROR)
				}
				isPlaying = false
			case track := <-identifySongResult:
				if track.OK && track.SpotifyID != "" {
					if spotifyClient == nil {
						display.ShowQR(track.SpotifyURL, 30)
						continue
					}
					err := spotifyClient.AddTrackToLibrary(track.SpotifyID)
					if err != nil {
						display.ShowStatus(ERROR)
						continue
					}
					log.Printf("[SPOTIFY] Added: %s - %s", track.Title, track.Artist)
					display.ShowStatus(ADDFAV)
				} else {
					display.ShowStatus(HUH)
				}
			}
		}
	}()

	playStation <- PickOne(favorite_stations)

	select {}

}

type Buff struct {
	FirstChunk             bool
	Sink                   *AudioSink
	PreviousStationProcess *os.Process
	Failtimer              *time.Timer
	DataStarted            chan bool
}

func (buff *Buff) Write(b []byte) (n int, err error) {
	if buff.FirstChunk {
		buff.Failtimer.Stop()
		if buff.PreviousStationProcess != nil {
			buff.PreviousStationProcess.Kill()
			log.Println("Stopped previous station")
		}
		buff.FirstChunk = false
		buff.DataStarted <- true
	}
	buff.Sink.Write(b)
	return len(b), nil
}

func play_station(station Station, sink *AudioSink, prevStation *os.Process, result chan StationProcess) {
	log.Printf("GET: %s", station.Name)
	buff := &Buff{
		true,
		sink,
		prevStation,
		time.NewTimer(30 * time.Second), // How long to wait for the next station to start before considering it a failure
		make(chan bool),
	}
	ffmpegCmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-i", station.URL, "-f", "wav", "-ar", "44100", "-ac", "2", "-")
	ffmpegOut, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err := ffmpegCmd.Start(); err != nil {
		panic(err)
	}
	go ffmpegCmd.Wait()
	go func() {
		_, err := io.Copy(buff, ffmpegOut)
		if err != nil {
			log.Printf("[%s] [STATION] Stream closed: %s", time.Now().Format("15:04:05"), station.Name)
		}
	}()
	stationProcess := StationProcess{station, ffmpegCmd, false}
	select {
	case <-buff.DataStarted:
		log.Printf("Streaming started: %s", station.Name)
		stationProcess.Started = true
		result <- stationProcess
	case <-buff.Failtimer.C:
		ffmpegCmd.Process.Kill()
		result <- stationProcess
	}
}

func add_favorite_station(newStation Station, currentStations []Station) error {
	// Check if station is already in favorites
	for _, station := range currentStations {
		if station.Name == newStation.Name {
			return nil
		}
	}
	currentStations = append(currentStations, newStation)
	return saveFavoriteStations(currentStations)
}

// NoOp is a function that accepts any number of arguments of any type with no return.
func noop(args ...interface{}) {
	// Do nothing
}

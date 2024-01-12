package main

import (
	"io"
	"log"
	"net/url"
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

	// Required by stations.go
	err := get_languages_from_file()
	if err != nil {
		log.Printf("[STATIONS] Failed to get languages: %s", err)
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

	// To enable Audd.io song identification, place your api token
	// in a file called `auddio_token.txt`
	b, err := os.ReadFile(AUDDIO_TOKEN_FILE)
	if err == nil && len(b) > 4 {
		IDENTIFY_ENABLED = true
		AUDDIO_API_KEY = string(b)
		log.Printf("[IDENTIFY] Enabled: %s\n", AUDDIO_API_KEY)
	}

	var spotifyClient *SpotifyClient
	var spotifyFunc func(*Display) (*SpotifyClient, error)

	_, err = os.ReadFile(OK_HTML)
	if err != nil {
		log.Printf("[SPOTIFY] `ok.html` missing.")
		os.Exit(1)
	}

	// To enable Spotify, create an empty file called `spotify_token.txt`
	// in the same directory as the binary.
	b, err = os.ReadFile(SPOTIFY_TOKEN_FILE)
	if err == nil {
		if len(b) <= 4 {
			log.Printf("[SPOTIFY] Enabled. Authenticating...")
			spotifyFunc = InitSpotifyClient
		} else {
			log.Printf("[SPOTIFY] Enabled. Reusing token.")
			spotifyFunc = RefreshSpotifyClient
		}
		spotifyClient, err = spotifyFunc(display)
		if err != nil {
			log.Printf("[SPOTIFY] Failed to init Spotify client: %s", err)
			os.Exit(1)
		}
	}

	display.ShowStatus <- SPLASH

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
				display.ShowStatus <- PLAYFAV
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
				display.ShowStatus <- SEARCH
				log.Printf("[STATIONS] Getting random station")
				station, err := get_random_station(currentStation.Station)
				if err != nil {
					log.Printf("[STATIONS] %s", err)
					isPlaying = false
					display.ShowStatus <- ERROR
					continue
				}
				playStation <- station
			case <-saveFav:
				display.ShowStatus <- ADDFAV
				err := add_favorite_station(currentStation.Station, favorite_stations)
				if err != nil {
					log.Printf("Failed to save favorite station: %s", err)
					continue
				}
				favorite_stations = append(favorite_stations, currentStation.Station)
				log.Printf("[FAVORITES] [%d] Added: %s", len(favorite_stations), currentStation.Station.Name)
			case <-identifySong:
				if !IDENTIFY_ENABLED {
					continue
				}
				go RecordAndIdentifySong(audioSink, identifySongResult)
				display.ShowStatus <- IDENTIFY
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
					display.ShowStatus <- PLAYING
					log.Printf("SET: %s", station.Name)
					currentStation = &station
				} else {
					log.Printf("[TIMEOUT] Station did not start")
					display.ShowStatus <- ERROR
				}
				isPlaying = false
			case track := <-identifySongResult:
				if track.OK {
					if spotifyClient == nil || track.SpotifyID == "" {
						escaped := url.QueryEscape(track.Title + " " + track.Artist)
						yt_seatrch_url := YOUTUBE_SEARCH + escaped
						display.ShowQR <- QR{yt_seatrch_url, 60, PLAYING}
						continue
					}
					err := spotifyClient.AddTrackToLibrary(track.SpotifyID)
					if err != nil {
						display.ShowStatus <- ERROR
						continue
					}
					log.Printf("[SPOTIFY] Added: %s - %s", track.Title, track.Artist)
					display.ShowStatus <- ADDFAV
				} else {
					display.ShowStatus <- HUH
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
			log.Printf("[STATION] Stream closed: %s", station.Name)
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

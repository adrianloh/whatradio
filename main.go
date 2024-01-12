package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/stianeikeland/go-rpio/v4"
)

var HOME = getExecutableDirectory()

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
		fmt.Printf("[DISPLAY] No status animations path: %s\n", STATUS_IMAGES_PATH)
		os.Exit(1)
	}

	// Required by stations.go
	err := get_languages_from_file()
	if err != nil {
		fmt.Printf("[STATIONS] Failed to get languages: %s\n", err)
		os.Exit(1)
	}

	// GPIO
	if err := rpio.Open(); err != nil {
		fmt.Printf("[GPIO] Failed to open GPIO: %s\n", err)
		os.Exit(1)
	}
	defer rpio.Close()

	// Init display ST7789
	display, err := NewDisplay()
	if err != nil {
		fmt.Printf("[DISPLAY] Failed to init display: %s\n", err)
		os.Exit(1)
	}

	// Check if ffmpeg is installed
	ffmpegCmd := exec.Command("ffmpeg")
	err = ffmpegCmd.Start()
	if err != nil {
		fmt.Printf("[FFMPEG] Failed to start: %s\n", err)
		os.Exit(1)
	}

	// To enable Audd.io song identification, place your api token
	// in a file called `auddio_token.txt`
	b, err := os.ReadFile(AUDDIO_TOKEN_FILE)
	if err == nil && len(b) > 4 {
		IDENTIFY_ENABLED = true
		AUDDIO_API_KEY = string(b)
		fmt.Printf("[IDENTIFY] Enabled: %s\n\n", AUDDIO_API_KEY)
	}

	var spotifyClient *SpotifyClient
	var spotifyFunc func(*Display) (*SpotifyClient, error)

	_, err = os.ReadFile(OK_HTML)
	if err != nil {
		fmt.Println("[SPOTIFY] `ok.html` missing.")
		os.Exit(1)
	}

	// To enable Spotify, create an empty file called `spotify_token.txt`
	// in the same directory as the binary.
	b, err = os.ReadFile(SPOTIFY_TOKEN_FILE)
	if err == nil {
		if len(b) <= 4 {
			fmt.Println("[SPOTIFY] Enabled. Authenticating...")
			spotifyFunc = InitSpotifyClient
		} else {
			fmt.Println("[SPOTIFY] Enabled. Reusing token.")
			spotifyFunc = RefreshSpotifyClient
		}
		spotifyClient, err = spotifyFunc(display)
		if err != nil {
			fmt.Printf("[SPOTIFY] Failed to init Spotify client: %s\n", err)
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
	nextStationResult := make(chan StationStream)

	identifySongResult := make(chan Track)

	// The current station, set after receiving the result on nextStationResult
	var currentStation = &StationStream{
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
					fmt.Println("[BUSY]")
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
					fmt.Println("[BUSY]")
					continue
				}
				isPlaying = true
				display.ShowStatus <- SEARCH
				fmt.Println("[STATIONS] Getting random station")
				station, err := get_random_station(currentStation.Station)
				if err != nil {
					fmt.Printf("[STATIONS] %s\n", err)
					isPlaying = false
					display.ShowStatus <- ERROR
					continue
				}
				playStation <- station
			case <-saveFav:
				display.ShowStatus <- ADDFAV
				err := add_favorite_station(currentStation.Station, favorite_stations)
				if err != nil {
					fmt.Printf("Failed to save favorite station: %s\n", err)
					continue
				}
				favorite_stations = append(favorite_stations, currentStation.Station)
				fmt.Printf("[FAVORITES] [%d] Added: %s\n", len(favorite_stations), currentStation.Station.Name)
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
				go NewStationStream(station, audioSink, currentStation, nextStationResult)
			case station := <-nextStationResult:
				if station.Started {
					display.ShowStatus <- PLAYING
					fmt.Printf("[ SET ]: %s\n", station.Name)
					currentStation = &station
					go station.Monitor(playRandom, display)
				} else {
					fmt.Println("[TIMEOUT] Station did not start")
					// If a station is still playing, then let the user manually try again
					if isAlive(currentStation.Process) {
						display.ShowStatus <- ERROR
					} else {
						// If nothing is playing, then try again automatically. Note, if stations
						// keep failing (in the case of the network being down), this branch
						// will loop indefinitely
						playRandom <- true
					}
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
					fmt.Printf("[SPOTIFY] Added: %s - %s\n", track.Title, track.Artist)
					display.ShowStatus <- ADDFAV
				} else {
					display.ShowStatus <- HUH
				}
			}
		}
	}()

	// playStation <- Station{
	// 	Name: "Silent Test Station",
	// 	URL:  "https://smack.s3.ap-southeast-1.amazonaws.com/pie_silence.mp3",
	// }
	playStation <- PickOne(favorite_stations)

	select {}

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

func isAlive(cmd *exec.Cmd) bool {
	// On Unix systems, FindProcess always succeeds and returns a Process
	// On Windows, FindProcess only succeeds if the process exists
	process, err := os.FindProcess(cmd.Process.Pid)
	if err != nil {
		return false
	}

	// Try to send signal 0 to check if the process is alive
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false
	}

	return cmd.ProcessState == nil || !cmd.ProcessState.Exited()
}

// NoOp is a function that accepts any number of arguments of any type with no return.
func noop(args ...interface{}) {
	// Do nothing
}

package main

// API docs: https://de1.api.radio-browser.info

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const LANGUAGES_FILE = "languages.txt"

var (
	BBC_ONE = Station{"BBC One",
		"0af24a33-1631-4c23-b09a-c1413d2c4fb0",
		"http://as-hls-ww-live.akamaized.net/pool_904/live/ww/bbc_radio_one/bbc_radio_one.isml/bbc_radio_one-audio%3d96000.norewind.m3u8",
		"pop",
	}

	STATION_SORT_FIELDS = []string{"clickcount", "votes", "clicktrend"}
	RADIO_SERVERS       = []string{`de1.api.radio-browser.info`, `at1.api.radio-browser.info`, `nl1.api.radio-browser.info`}
	LANGUAGES           = []string{}

	last_search_time             = time.Now()
	last_stations_search_results = []Station{}
)

// https://de1.api.radio-browser.info/json/stations/byuuid/0af24a33-1631-4c23-b09a-c1413d2c4fb0
type Station struct {
	Name string `json:"name"`
	UUID string `json:"stationuuid"`
	URL  string `json:"url_resolved"`
	Tags string `json:"tags"`
}

func get_languages_from_file() error {
	file, err := os.Open(LANGUAGES_FILE)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			LANGUAGES = append(LANGUAGES, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if len(LANGUAGES) == 0 {
		return errors.New("No languages found. Check `languages.txt``")
	}

	return nil
}

func get_radio_servers() error {
	service := "api"
	protocol := "tcp"
	domain := "radio-browser.info"
	// Perform DNS SRV lookup
	_, srvRecords, err := net.LookupSRV(service, protocol, domain)
	if err != nil || len(srvRecords) == 0 {
		return fmt.Errorf("Error during SRV lookup: %v\n", err)
	}
	for _, srv := range srvRecords {
		// srv.Target looks like `de1.api.radio-browser.info.`
		RADIO_SERVERS = append(RADIO_SERVERS, srv.Target[:len(srv.Target)-1])
	}
	return nil
}

// https://de1.api.radio-browser.info/#Advanced_station_search
// https://jsonviewer.stack.hu/#http://de1.api.radio-browser.info/json/stations/search?limit=3&order=clickcount&language=english&reverse=true
func parse_query_url() string {
	s := fmt.Sprintf("https://%s/json/stations/search?", PickOne(RADIO_SERVERS))
	s += fmt.Sprintf("limit=%d&", 50)
	s += fmt.Sprintf("order=%s&", PickOne(STATION_SORT_FIELDS))
	s += fmt.Sprintf("language=%s&", PickOne(LANGUAGES))
	s += "&reverse=true"
	return s
}

func parse_query_url_with_language(station int, language string) string {
	s := fmt.Sprintf("https://%s/json/stations/search?", RADIO_SERVERS[station])
	s += fmt.Sprintf("limit=%d&", 10)
	s += fmt.Sprintf("order=%s&", PickOne(STATION_SORT_FIELDS))
	s += fmt.Sprintf("language=%s&", language)
	s += "&reverse=true"
	return s
}

func get_station_by_uuid(uuid string) (Station, error) {
	res, err := http.Get("https://" + PickOne(RADIO_SERVERS) + ".api.radio-browser.info/json/stations/byuuid/" + uuid)
	if err != nil {
		return Station{}, err
	}
	defer res.Body.Close()
	stations := json_to_stations(res)
	if err != nil || len(stations) == 0 {
		return Station{}, errors.New("No station matching UUID: " + uuid)
	}
	return stations[0], nil
}

func get_random_station(currentStation Station) (Station, error) {
	stations, err := search_stations()
	if err != nil { // Note: An empty list of stations is also an error
		return Station{}, err
	}
	Shuffle(stations)
	for _, station := range stations {
		if station.UUID != currentStation.UUID {
			return station, nil
		}
	}
	return BBC_ONE, nil
}

func search_stations() ([]Station, error) {

	if time.Since(last_search_time) < 300*time.Second && len(last_stations_search_results) > 0 {
		return last_stations_search_results, nil
	}

	var selected_languages []string

	if len(LANGUAGES) > 3 {
		Shuffle(LANGUAGES)
		selected_languages = LANGUAGES[:3]
	} else {
		selected_languages = LANGUAGES
	}

	results := make(chan []Station)

	for i, language := range selected_languages {
		go func(station int, language string) {
			query_url := parse_query_url_with_language(i, language)
			resp, err := http.Get(query_url)
			if err != nil {
				results <- []Station{}
				return
			}
			results <- json_to_stations(resp)
			fmt.Println("[STATIONS] Fetched language: " + language)
		}(i, language)
	}

	stations := []Station{}

	for i := 0; i < len(selected_languages); i++ {
		newStations := <-results
		stations = append(stations, newStations...)
	}

	if len(stations) == 0 {
		return stations, errors.New("No stations found")
	}

	Shuffle(stations)

	last_search_time = time.Now()
	last_stations_search_results = stations

	return stations, nil

}

func json_to_stations(resp *http.Response) []Station {
	defer resp.Body.Close()
	bytes, _ := io.ReadAll(resp.Body)
	stations := []Station{}
	json.Unmarshal(bytes, &stations)
	return stations
}

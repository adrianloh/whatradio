package main

// API docs: https://de1.api.radio-browser.info

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
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

	STATION_SORT_FIELDS = []string{"clickcount", "votes", "clicktrend", "random"}
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

func parse_query_url(stationURL string, limit int, sortfield string, language string) string {
	s := fmt.Sprintf("https://%s/json/stations/search?", stationURL)
	s += fmt.Sprintf("limit=%d&", limit)
	s += fmt.Sprintf("order=%s&", sortfield)
	s += fmt.Sprintf("language=%s&", language)
	if rand.Float64() < 0.5 {
		s += "&reverse=true"
	}
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

	stationsResult := make(chan []Station)

	selectedLanguages := []string{}
	Shuffle(LANGUAGES)
	if len(LANGUAGES) > 3 {
		selectedLanguages = LANGUAGES[:3]
	} else {
		for len(selectedLanguages) < len(RADIO_SERVERS) {
			selectedLanguages = append(selectedLanguages, PickOne(LANGUAGES))
		}
	}

	for i, language := range selectedLanguages {
		go func(i int, language string) {
			server := RADIO_SERVERS[i]
			query_url := parse_query_url(
				server,
				10,
				PickOne(STATION_SORT_FIELDS),
				language)
			stations, err := search_stations(query_url)
			if err != nil {
				log.Printf("[%s] Failed: %s", server, err)
			}
			stationsResult <- stations
		}(i, language)
	}

	stationResults := []Station{}

	for i := 0; i < len(RADIO_SERVERS); i++ {
		stations := <-stationsResult
		if len(stations) > 0 {
			stationResults = append(stationResults, stations...)
		}
	}

	if len(stationResults) == 0 {
		return Station{}, errors.New("No stations returned from search")
	}

	Shuffle(stationResults)

	for _, station := range stationResults {
		if station.UUID != currentStation.UUID {
			return station, nil
		}
	}

	return Station{}, errors.New("Failed to get random station")

}

func search_stations(query_url string) ([]Station, error) {
	// if time.Since(last_search_time) < 60*time.Second && len(last_stations_search_results) > 0 {
	// 	return last_stations_search_results, nil
	// }
	var stations []Station

	resp, err := http.Get(query_url)
	if err != nil {
		return nil, err
	}
	stations = json_to_stations(resp)
	if len(stations) == 0 {
		return nil, errors.New("No stations found")
	}
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

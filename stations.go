package main

// API docs: https://de1.api.radio-browser.info

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

var (
	BBC_ONE = Station{"BBC One",
		"0af24a33-1631-4c23-b09a-c1413d2c4fb0",
		"http://as-hls-ww-live.akamaized.net/pool_904/live/ww/bbc_radio_one/bbc_radio_one.isml/bbc_radio_one-audio%3d96000.norewind.m3u8",
		"pop",
	}
)

// https://de1.api.radio-browser.info/json/stations/byuuid/0af24a33-1631-4c23-b09a-c1413d2c4fb0
// https://jsonviewer.stack.hu/#http://de1.api.radio-browser.info/json/stations/search?limit=3&order=clickcount&language=english&reverse=true
type Station struct {
	Name string `json:"name"`
	UUID string `json:"stationuuid"`
	URL  string `json:"url_resolved"`
	Tags string `json:"tags"`
}

var STATION_SORT_FIELDS = []string{"clickcount", "votes", "clicktrend"}
var RADIO_SERVERS = []string{`de1.api.radio-browser.info`, `at1.api.radio-browser.info`, `nl1.api.radio-browser.info`}
var LANGUAGES = []string{
	"japanese",
	"italian",
	"cantonese",
	"spanish",
	"hindi",
	"chinese",
	"german",
	"french",
	"swedish",
	"japanese",
	"italian",
	"russian",
	"english",
	"german",
	"french",
	"swedish",
}

var (
	last_search_time             = time.Now()
	last_stations_search_results = []Station{}
)

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
func parse_query_url() string {
	s := fmt.Sprintf("https://%s/json/stations/search?", PickOne(RADIO_SERVERS))
	s += fmt.Sprintf("limit=%d&", 50)
	s += fmt.Sprintf("order=%s&", PickOne(STATION_SORT_FIELDS))
	s += fmt.Sprintf("language=%s&", PickOne(LANGUAGES))
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
	if time.Since(last_search_time) < 60*time.Second && len(last_stations_search_results) > 0 {
		return last_stations_search_results, nil
	}
	var stations []Station
	query_url := parse_query_url()

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
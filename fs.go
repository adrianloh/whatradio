package main

import (
	"encoding/json"
	"os"
)

var FAVORITES_FILE = "favstations.json"

func getFavoriteStations() []Station {
	fileData, err := os.ReadFile(FAVORITES_FILE)
	if err != nil {
		return []Station{
			{"BBC One",
				"0af24a33-1631-4c23-b09a-c1413d2c4fb0",
				"http://as-hls-ww-live.akamaized.net/pool_904/live/ww/bbc_radio_one/bbc_radio_one.isml/bbc_radio_one-audio%3d96000.norewind.m3u8",
				"pop",
			},
			{
				"106,7 Rockklassiker",
				"9642ad8b-0601-11e8-ae97-52543be04c81",
				"http://edge-bauerse-02-thn.sharp-stream.com/rockklassiker_instream_se_mp3?ua=WEB&",
				"classic rock",
			},
		}
	}
	stations := []Station{}
	json.Unmarshal(fileData, &stations)
	return stations
}

func saveFavoriteStations(stations []Station) error {
	fileData, err := json.Marshal(stations)
	if err != nil {
		return err
	}
	return os.WriteFile(FAVORITES_FILE, fileData, 0644)
}

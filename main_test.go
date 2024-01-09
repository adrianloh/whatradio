package main

import (
	"fmt"
	"net"
	"testing"
)

func TestAuddioIdentify(t *testing.T) {
	t.SkipNow()
	track, err := identify_song_file("testfiles/trouble.mp3")
	if err != nil {
		t.Errorf("Failed to identify song: %s", err)
	}
	fmt.Printf("[%s] Identified song: %s - %s @ %s\n", track.Spotify.ID, track.Artist, track.Title, track.Spotify.External_URLS["spotify"])
	_, err = identify_song_file("testfiles/fdau.mp3")
	if err == nil {
		t.Errorf("Identified song that should not exist")
	}
}

func TestDNS(t *testing.T) {
	service := "api"
	protocol := "tcp"
	domain := "radio-browser.info"

	// Perform SRV lookup
	_, srvRecords, err := net.LookupSRV(service, protocol, domain)
	if err != nil {
		fmt.Printf("Error during SRV lookup: %v\n", err)
		return
	}

	// Print SRV records
	for _, srv := range srvRecords {
		fmt.Printf("Host: %s, Port: %d, Priority: %d, Weight: %d\n",
			srv.Target, srv.Port, srv.Priority, srv.Weight)
	}
}

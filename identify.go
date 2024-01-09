package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

var (
	IDENTIFY_ENABLED = false
	AUDDIO_API_KEY   = ""
)

const (
	AUDDIO_TOKEN_FILE = "auddio_token.txt"
	AUDDIO_GATEWAY    = "https://api.audd.io/"
	YOUTUBE_SEARCH    = "https://www.youtube.com/results?search_query="
)

type ApiResponse struct {
	Status string  `json:"status"`
	Result *Result `json:"result"`
}

type Result struct {
	Title   string  `json:"title"`
	Artist  string  `json:"artist"`
	Spotify Spotify `json:"spotify"`
}

type Spotify struct {
	ID            string            `json:"id"`
	External_URLS map[string]string `json:"external_urls"`
}

type Track struct {
	Title      string
	Artist     string
	SpotifyID  string
	SpotifyURL string
	OK         bool
}

func RecordAndIdentifySong(audioSink *AudioSink, identifySongResult chan Track) {
	track := Track{OK: false}
	fmt.Println("[IDENTIFY] Recording sample")
	recordedClioPath, err := audioSink.RecordSample()
	if err != nil {
		fmt.Printf("[RECORD] Failed: %s\n", err)
		identifySongResult <- track
		return
	}
	fmt.Printf("[RECORD] Saved: %s", recordedClioPath)
	result, err := identify_song_file(recordedClioPath)
	if err != nil {
		fmt.Printf("[IDENTIFY] Failed: %s\n", err)
		identifySongResult <- track
		return
	}
	// Note that the result contains Title and Artist, but doesn't gurantee that we have a Spotify ID
	fmt.Printf("[IDENTIFY] [%s] %s - %s\n", result.Spotify.ID, result.Title, result.Artist)
	track.OK = true
	track.Title = result.Title
	track.Artist = result.Artist
	track.SpotifyID = result.Spotify.ID
	if result.Spotify.External_URLS != nil {
		track.SpotifyURL = result.Spotify.External_URLS["spotify"]
	}
	identifySongResult <- track
}

func identify_song_file(fp string) (result Result, err error) {

	// API reference: https://docs.audd.io/

	file, err := os.Open(fp)
	if err != nil {
		return result, err
	}
	defer file.Close()

	// Prepare the form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Add the file to the form
	fw, _ := w.CreateFormFile("file", fp)
	io.Copy(fw, file)
	w.WriteField("api_token", AUDDIO_API_KEY)
	w.WriteField("return", "spotify")
	w.Close()

	// Create the HTTP request
	req, _ := http.NewRequest("POST", AUDDIO_GATEWAY, &b)
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return result, fmt.Errorf("[auddio] API responded with: %d", res.StatusCode)
	}

	// Read the response
	byteBody, _ := io.ReadAll(res.Body)

	var apiResponse ApiResponse
	if err := json.Unmarshal(byteBody, &apiResponse); err != nil {
		return result, fmt.Errorf("[auddio] failed to decode JSON")
	}

	if apiResponse.Result == nil {
		return result, fmt.Errorf("[auddio] identification failed")
	}

	return *apiResponse.Result, nil

}

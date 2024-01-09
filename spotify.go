package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

var SPOTIFY_TOKEN_FILE = "spotify_token.txt"

const (
	redirectURI         = "http://raspberrypi.local:54541/callback"
	spotifyClientId     = "d242f9bcdc2f474abcbd667eae3ff7e9"
	spotifyClientSecret = "a8dd1bb931f042e3a0de85489e3e428c"
)

var (
	auth  = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserLibraryModify)
	ch    = make(chan *spotify.Client)
	state = "abc123"
)

type SpotifyClient struct {
	client       *spotify.Client
	refreshToken string
}

func (s *SpotifyClient) AddTrackToLibrary(trackID string) error {
	if err := s.client.AddTracksToLibrary(spotify.ID(trackID)); err != nil {
		return fmt.Errorf("failed to add track to library: %v", err)
	}
	return nil
}

func NewSpotifyClient(display *Display) (*SpotifyClient, error) {
	spotifyClient := &SpotifyClient{}
	auth.SetAuthInfo(spotifyClientId, spotifyClientSecret)
	refreshTokenBytes, _ := os.ReadFile(SPOTIFY_TOKEN_FILE)
	spotifyClient.refreshToken = string(refreshTokenBytes)
	token, err := refreshToken(spotifyClient.refreshToken)
	if err != nil {
		return spotifyClient, fmt.Errorf("Error refreshing token: %v\n", err)
	}
	client := auth.NewClient(token)
	spotifyClient.client = &client
	return spotifyClient, nil
}

func _NewSpotifyClient(display *Display) *SpotifyClient {

	spotifyClient := &SpotifyClient{}

	auth.SetAuthInfo(spotifyClientId, spotifyClientSecret)
	// Try to read the refresh token from the file
	refreshTokenBytes, err := os.ReadFile(SPOTIFY_TOKEN_FILE)
	if err == nil && len(refreshTokenBytes) > 0 {
		spotifyClient.refreshToken = string(refreshTokenBytes)
		// Refresh token is available, use it to get a new access token
		token, err := refreshToken(spotifyClient.refreshToken)
		if err != nil {
			fmt.Printf("Error refreshing token: %v\n", err)
		} else {
			client := auth.NewClient(token)
			spotifyClient.client = &client
			return spotifyClient
		}
	}

	// Start HTTP server and handle authentication if refresh token is not available
	http.HandleFunc("/callback", completeAuth)
	go http.ListenAndServe(":54541", nil)

	url := auth.AuthURL(state)
	display.ShowQR(url, 0)
	fmt.Println("fmtin to Spotify:\n", url)

	client := <-ch

	spotifyClient.client = client

	return spotifyClient

}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	token, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatalf("couldn't get token: %v", err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("state mismatch: %v != %v", st, state)
	}

	client := auth.NewClient(token)
	fmt.Fprintf(w, "fmtin Completed!")

	// Save the refresh token to a file
	err = os.WriteFile(SPOTIFY_TOKEN_FILE, []byte(token.RefreshToken), 0600)
	if err != nil {
		log.Fatalf("failed to save refresh token: %v", err)
	}

	ch <- &client
}

func refreshToken(refreshToken string) (*oauth2.Token, error) {
	config := &oauth2.Config{
		ClientID:     spotifyClientId,
		ClientSecret: spotifyClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.spotify.com/authorize",
			TokenURL: "https://accounts.spotify.com/api/token",
		},
		RedirectURL: redirectURI,
		Scopes:      []string{spotify.ScopeUserLibraryModify},
	}
	tokenSource := config.TokenSource(context.Background(), &oauth2.Token{
		RefreshToken: refreshToken,
	})

	newToken, err := tokenSource.Token() // This automatically refreshes the token
	if err != nil {
		return nil, fmt.Errorf("could not refresh token: %v", err)
	}
	return newToken, nil
}

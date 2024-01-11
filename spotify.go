package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

const (
	SPOTIFY_TOKEN_FILE  = "spotify_token.txt"
	OK_HTML             = "ok.html"
	redirectURI         = "http://raspberrypi.local:54541/callback"
	spotifyClientId     = "d242f9bcdc2f474abcbd667eae3ff7e9"
	spotifyClientSecret = "a8dd1bb931f042e3a0de85489e3e428c"
)

var (
	auth        = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserLibraryModify)
	spotifyOK   = make(chan *spotify.Client)
	spotifyFail = make(chan error)
	state       = "abc123"
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

func RefreshSpotifyClient(display *Display) (*SpotifyClient, error) {
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

func InitSpotifyClient(display *Display) (*SpotifyClient, error) {

	spotifyClient := &SpotifyClient{}

	auth.SetAuthInfo(spotifyClientId, spotifyClientSecret)

	// Start HTTP server and handle authentication if refresh token is not available
	http.HandleFunc("/callback", completeAuth)
	go http.ListenAndServe(":54541", nil)

	url := auth.AuthURL(state)
	display.ShowQR <- QR{url, 0, PERMANENT}
	fmt.Println("login to Spotify:\n", url)

	select {
	case <-spotifyFail:
		return spotifyClient, fmt.Errorf("failed to authenticate with Spotify")
	case client := <-spotifyOK:
		spotifyClient.client = client
	}
	return spotifyClient, nil

}

func completeAuth(w http.ResponseWriter, r *http.Request) {

	bytes, _ := os.ReadFile("ok.html")
	htmlOK := string(bytes)
	htmlNoOK := strings.Replace(htmlOK, "😃", "😪", 1)

	token, err := auth.Token(state, r)
	if err != nil {
		fmt.Fprintf(w, htmlNoOK)
		spotifyFail <- fmt.Errorf("couldn't get token: %v\n", err)
		return
	}
	if st := r.FormValue("state"); st != state {
		fmt.Fprintf(w, htmlNoOK)
		spotifyFail <- fmt.Errorf("state mismatch: %v != %v\n", st, state)
		return
	}

	// Save the refresh token to a file
	err = os.WriteFile(SPOTIFY_TOKEN_FILE, []byte(token.RefreshToken), 0600)
	if err != nil {
		fmt.Fprintf(w, htmlNoOK)
		spotifyFail <- fmt.Errorf("failed to save refresh token: %v\n", err)
		return
	}

	client := auth.NewClient(token)
	fmt.Fprintf(w, htmlOK)
	spotifyOK <- &client

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

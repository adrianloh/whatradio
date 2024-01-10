# WhatRadio

Transform your Raspberry Pi Zero with the [Pimoroni DAC](https://shop.pimoroni.com/products/pirate-audio-line-out?variant=31189750546515) into an internet radio.

## Base Setup
To set up WhatRadio, 
1. Place the `build` directory in `/home/pi`
2. Rename the directory to `whatradio` e.g. `/home/pi/whatradio`
3. Run `sudo ./install.sh`

## Song Recognition
Identify the song currently playing (and even add it to your Spotify library!) automagically.

#### Setup
1. Get an api key from [audd.io](https://audd.io)
2. Place the key into a file called `auddio_token.txt` in `/home/pi/whatradio`
3. Restart the radio.

When a song is successfully matched, a QR code will appear on the screen that looks up the song on Youtube!

#### Add To Spotify
1. Create an *empty file* called `spotify_token.txt` in `/home/pi/whatradio`
2. Restart the radio.
3. You will be prompted with a QR code on the screen.
4. Follow the QR to finish authentication.

## Languages
Available languages are contained in `languages.txt`. You can edit this file to set what languages you would like to hear on the radio.

## Usage

| Button | Function |
|----------|----------|
|   A  |   Does nothing (for now)  |
|   B  |   Toggle mute/unmute  |
|   X (press)  |   Play a random station  |
|   X (hold)  |   Identify current song and add it to Spotify  |
|   Y (press)  |   Play a station from favorites  |
|   Y (hold)  |   Add current station to favorites  |

Notes:

1. For best experience, run this on a Raspberry Pi Zero 2 W
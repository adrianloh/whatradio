# WhatRadio

Welcome to WhatRadio, the coolest internet radio experience designed for Raspberry Pi Zero 2. This isn't just a radio; it's your gateway to a world of music, powered by the slick [Pimoroni DAC](https://shop.pimoroni.com/products/pirate-audio-line-out?variant=31189750546515). Transform your Raspberry Pi into a music wizard, playing tunes from across the globe, and even recognizing songs like magic!

## Base Setup
To set up WhatRadio, 
1. Place the `build` directory in `/home/pi`
2. Rename the directory to `whatradio` e.g. `/home/pi/whatradio`
3. Run `sudo ./install.sh`

If you're running headless, `rsync` the folder to your Pi:
```
ssh pi@raspberrypi.local mkdir /home/pi/whatradio
rsync -avz build/ pi@raspberrypi.local:/home/pi/whatradio/
```

## Musical Sherlock: Song Recognition
Guess the song playing? Old school! WhatRadio identifies it instantly – and can even add it to your Spotify library. 🎶

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

When a song is matched, it will automatically be added to your Spotify Liked

## Speak My Language!
Tune in to the world! Edit `languages.txt` to pick languages for your global music journey.

## Controls

| Button | Function |
|----------|----------|
|   A  |   SHIFT  |
|   B  |   Toggle mute/unmute  |
|   X (press)  |   Play a random station  |
|   X (hold)  |   Identify current song and add it to Spotify  |
|   Y (press)  |   Play a station from favorites  |
|   Y (hold)  |   Add current station to favorites  |
|   Y (hold) + SHIFT  |   Remove station from favorites  |

### Test Platform:

1. For best experience, run this on a Raspberry Pi Zero 2 W. To run on the Zero 1, you'll have to re-compile the binary with:
```
export GOOS=linux
export GOARCH=arm
export GOARM=6  # Zero 2 would be `7`
```
2. OS: Raspberry Pi OS (Legacy, 64-bit)
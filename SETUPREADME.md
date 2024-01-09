Add these lines to `/boot/config.txt`

```
# Enable display
dtparam=spi=on

# PCM5102 DAC
dtoverlay=hifiberry-dac
gpio=25=op,dh

# go-rpio
dtoverlay=gpio-no-irq
```

# ALSA

`aplay -L`

```
null
    Discard all samples (playback) or generate zero samples (capture)
hw:CARD=sndrpihifiberry,DEV=0
    snd_rpi_hifiberry_dac, HifiBerry DAC HiFi pcm5102a-hifi-0
    Direct hardware device without any conversions
plughw:CARD=sndrpihifiberry,DEV=0
    snd_rpi_hifiberry_dac, HifiBerry DAC HiFi pcm5102a-hifi-0
    Hardware device with all software conversions
default:CARD=sndrpihifiberry
    snd_rpi_hifiberry_dac, HifiBerry DAC HiFi pcm5102a-hifi-0
    Default Audio Device
sysdefault:CARD=sndrpihifiberry
    snd_rpi_hifiberry_dac, HifiBerry DAC HiFi pcm5102a-hifi-0
    Default Audio Device
dmix:CARD=sndrpihifiberry,DEV=0
    snd_rpi_hifiberry_dac, HifiB
```

`speaker-test -D plughw:CARD=sndrpihifiberry,DEV=0 -c 2 -twav`


/etc/asound.conf
```
pcm.!default {
    type             plug
    slave.pcm       "softvol"
}

pcm.softvol {
    type softvol
    slave {
        pcm "plughw:CARD=sndrpihifiberry,DEV=0"
    }
    control {
        name "Master"
        card sndrpihifiberry
    }
}
```

`amixer set Master 20%-`

/lib/systemd/system/whatradio.service
```
[Unit]
Description=What Radio
After=network.target

[Service]
ExecStart=/home/pi/whatradio/whatradio
WorkingDirectory=/home/pi/whatradio
StandardOutput=inherit
StandardError=inherit

[Install]
WantedBy=multi-user.target
```

sudo systemctl daemon-reload
sudo systemctl enable whatradio

To monitor the process:

`journalctl -fu whatradio`
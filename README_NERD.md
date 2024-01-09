# ALSA

To get a list of playback devices, run `aplay -L`. Which outputs:
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

What `aplay` uses is determined by `/etc/asound.conf`. Any changes you make to this file is reflected immedietly on the next call to `aplay`.

To test, run:
```
speaker-test -D plughw:CARD=sndrpihifiberry,DEV=0 -c 2 -twav
```
    
# SYSTEMD

To `start|stop` the `whatradio` process manually:
```
sudo systemctl (start|stop) whatradio
```

To monitor the process:
```
journalctl -fu whatradio
```
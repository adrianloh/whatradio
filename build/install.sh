#!/bin/bash

# Check if the script is running in /home/pi/whatradio
if [ "$(pwd)" != "/home/pi/whatradio" ]; then
    echo "This script must be run from /home/pi/whatradio directory" 1>&2
    exit 1
fi

chmod +x whatradio

# Check for root permission
if [ "$(id -u)" != "0" ]; then
   echo "This script must be run as root" 1>&2
   exit 1
fi

# Install ffmpeg
sudo apt-get install -y ffmpeg

# Content to be added to /boot/config.txt
boot_config_content="

# Boot faster
disable_splash=1
boot_delay=0

# Enable ST7789 display
dtparam=spi=on

# PCM5102 DAC
dtoverlay=hifiberry-dac
gpio=25=op,dh

# Required by \`rpio-go\`
dtoverlay=gpio-no-irq
"

# Append content to /boot/config.txt
echo "$boot_config_content" | tee -a /boot/config.txt

# Content for /etc/asound.conf
asound_conf_content="
pcm.!default {
    type             plug
    slave.pcm       \"softvol\"
}

pcm.softvol {
    type softvol
    slave {
        pcm \"plughw:CARD=sndrpihifiberry,DEV=0\"
    }
    control {
        name \"Master\"
        card sndrpihifiberry
    }
}
"

# Write content to /etc/asound.conf
echo "$asound_conf_content" > /etc/asound.conf

# Content for whatradio.service
whatradio_service_content="
[Unit]
Description=What Radio
After=network-online.target wpa_supplicant.service
Wants=network-online.target

[Service]
ExecStart=/home/pi/whatradio/whatradio
WorkingDirectory=/home/pi/whatradio
StandardOutput=inherit
StandardError=inherit
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
"

# Write content to whatradio.service
echo "$whatradio_service_content" > /lib/systemd/system/whatradio.service

# Reload systemd manager configuration
systemctl daemon-reload

# Enable whatradio service
systemctl enable whatradio

sudo systemctl disable hciuart.service
sudo systemctl disable  raspi-config.service
sudo systemctl disable keyboard-setup.service
sudo systemctl disable ModemManager.service
sudo systemctl disable systemd-timesyncd.service
sudo apt-get install -y ntpdate
sudo ntpdate pool.ntp.org

echo "Setup completed."

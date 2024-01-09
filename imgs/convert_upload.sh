#!/bin/bash

# Check if an argument was provided
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <filename>"
    exit 1
fi

# Check if the specified file exists
if [ ! -f "$1" ]; then
    echo "File not found: $1"
    exit 1
fi

# Extract filename without extension
filename=$(basename -- "$1")
extension="${filename##*.}"
filename="${filename%.*}"

# Run the ImageMagick convert command
convert "$1" -coalesce -resize 240x240 -background transparent -gravity center -extent 240x240 -format gif "../build/gifs/${filename}_%03d.gif"

scp ../build/gifs/"${filename}_"*.gif pi@192.168.1.33:/home/pi/gifs/
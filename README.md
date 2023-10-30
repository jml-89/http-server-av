# httpfileserve
A less simple more featured webserver for a directory of files

## Features
Creates thumbnails for video files
Parses metadata from media files
Has a search function which searches filenames, metadata, et cetera.

## Curiosities
Uses the ffmpeg C API for reasons
Managing allocations in Go is a little easier than C but not by much
I believe some code paths leak memory

## Compatibility
Linux

## Requirements
Go
ffmpeg development libraries (usually ffmpeg-dev in package manager)

## Installation
go get
go install


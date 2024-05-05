Web-serve a directory of video and/or audio and/or image files   
It's like http-server, but tuned for media files   

# Usage
Install it on your PATH  
Navigate to the directory you want to serve  
Run `http-server-av`  
By default it will serve on port 8080, you can change that with the --port argument

# Features
Creates thumbnails for video files  
Parses metadata from media files  
Has a search function which searches filenames, metadata, et cetera.  
Simple duplicate video detection (comparing thumbnails)  
  
# Curiosities
Uses ffmpeg's libav C API rather than shelling out an ffmpeg process  
Managing manual memory allocations in Go is a little easier than C but not by much  
I believe some code paths leak memory -- barely any, seems to be in the webp code somewhere  

# Compatibility
Linux  
Could work on Windows too, just don't have a Windows machine to test on

# Requirements
Go  
ffmpeg development libraries (usually ffmpeg-dev in package manager)  

# Installation
Clone this repo, then run the following from inside it   
`go get`   
`go install`   
   
Or just do    
`go install github.com/jml-89/http-server-av`   


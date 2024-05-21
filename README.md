Web-serve a directory of video and/or audio and/or image files   
It's like http-server, but tuned for media files   
    
Compiles to a single binary, nothing else to deploy... but has dependencies right now   
Beyond basic libc (cgo) it also requires OpenCV and ffmpeg installed   
OpenCV can be statically compiled and linked in though, with some effort   
And ffmpeg can be statically linked already   
Potentially can static everything, musl/opencv/ffmpeg/sqlite3 and get a no-dep binary   
But have to investigate that some more

# Usage
Install it on your PATH  
Navigate to the directory you want to serve  
Run `http-server-av`  
By default it will serve on port 8080, you can change that with the --port argument   

# Features
Creates thumbnails for video files  
Will try to create the "best" thumbnail it can by finding faces   
Parses metadata from media files  
Has a search function which searches filenames, metadata, et cetera.  
Simple duplicate video detection (comparing thumbnails)  
  
# Quirks
Uses ffmpeg's libav C API rather than shelling out an ffmpeg process  
Managing manual memory allocations in Go is a little easier than C but not by much  
I believe some code paths leak memory -- barely any, seems to be in the webp code somewhere  
    
# Compatibility
Linux  
Could work on Windows too, just don't have a Windows machine to test on

# Requirements 
## Build 
Go 1.22+   
ffmpeg development libraries (usually ffmpeg-dev in package manager)  
opencv development libraries (usually libopencv-dev in package manager)   
C compiler   
C++ compiler   
## Deploy
libc, ffmpeg, OpenCV on target system   

# Installation
Clone this repo, then run the following from inside it   
`go get`   
`go install`   
   
Or just do    
`go install github.com/jml-89/http-server-av@latest`   


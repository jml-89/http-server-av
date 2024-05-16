//Wrapper for thumbnailer.cpp

package av

/*
#cgo CXXFLAGS: -Wall -Wextra -Wfatal-errors -Wno-deprecated-enum-enum-conversion 
#cgo CXXFLAGS: -std=c++23 -I/usr/include/opencv4
#cgo LDFLAGS: -lopencv_core -lopencv_dnn -lopencv_videoio -lopencv_imgproc -lopencv_imgcodecs

#include "stdlib.h"
#include "thumbnailer.h"

size_t face_size(void) { return sizeof(face); }
*/
import "C"

import (
	"unsafe"
	"os"
	"log"
)

import _ "embed"
//go:embed weights/face-quality-assessment.onnx
var netAssess []byte
//go:embed weights/yolov8n-face.onnx
var netDetect []byte

type Thumbnailer struct {
	tmb *C.thumbnailer
}

type Face struct {
	Area int64
	Confidence float32
	Quality float32
}

func NewThumbnailer(numThreads int) (*Thumbnailer, error) {
	C.cv_set_num_threads(C.int(numThreads))

	fileDetect, err := os.CreateTemp(os.TempDir(), "http-server-av.yolov8n-face.*.onnx")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer os.Remove(fileDetect.Name())

	fileAssess, err := os.CreateTemp(os.TempDir(), "http-server-av.face-quality-assessment.*.onnx")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer os.Remove(fileAssess.Name())

	err = os.WriteFile(fileDetect.Name(), netDetect, 0666)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	err = os.WriteFile(fileAssess.Name(), netAssess, 0666)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	m1 := C.CString(fileDetect.Name())
	defer C.free(unsafe.Pointer(m1))

	m2 := C.CString(fileAssess.Name())
	defer C.free(unsafe.Pointer(m2))

	tmber := C.thumbnailer_init(m1, m2)
	return &Thumbnailer { tmb: tmber }, nil
}

func (t *Thumbnailer) Close() {
	C.thumbnailer_free(t.tmb)
}

func (t *Thumbnailer) RunImageBuf(image []byte) ([]Face, error) {
	buf := C.CBytes(image)
	defer C.free(buf)

	//start := time.Now()
	finds := C.thumbnailer_run_image_buf(t.tmb, (*C.uchar)(buf), (C.size_t)(len(image)))
	//end := time.Now()
	//log.Printf("\t%s\n", end.Sub(start))

	faces := make([]Face, int(finds.len))
	for i := 0; i < int(finds.len); i++ {
		faces[i] = Face { 
			Area: int64(finds.faces[i].area),
			Confidence: float32(finds.faces[i].confidence), 
			Quality: float32(finds.faces[i].quality),
		}
	}

	return faces, nil
}


func (t *Thumbnailer) RunImage(path_image string) ([]Face, error) {
	pin := C.CString(path_image)
	defer C.free(unsafe.Pointer(pin))

	finds := C.thumbnailer_run_image(t.tmb, pin)

	faces := make([]Face, int(finds.len))
	for i := 0; i < int(finds.len); i++ {
		faces[i] = Face { 
			Area: int64(finds.faces[i].area),
			Confidence: float32(finds.faces[i].confidence), 
			Quality: float32(finds.faces[i].quality),
		}
	}

	return faces, nil
}

func (t *Thumbnailer) Run(path_video string, path_thumbnail string, probes int) error {
	pin := C.CString(path_video)
	defer C.free(unsafe.Pointer(pin))

	pot := C.CString(path_thumbnail)
	defer C.free(unsafe.Pointer(pot))

	C.thumbnailer_run(t.tmb, pin, pot, C.int(probes))

	return nil
}


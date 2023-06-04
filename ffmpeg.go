package main

/*
#cgo CFLAGS: -Wall
#cgo LDFLAGS: -lavformat -lavutil -lavcodec -lavfilter

#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <inttypes.h>

#include <libavformat/avformat.h>
#include <libavcodec/avcodec.h>
#include <libavutil/dict.h>
#include <libavutil/pixdesc.h>
#include <libavutil/opt.h>
#include <libavfilter/buffersink.h>
#include <libavfilter/buffersrc.h>

int get_pix_fmt(enum AVPixelFormat* fmts, enum AVPixelFormat hope) {
	for (int i = 0; fmts[i] != -1; i++) {
	if (fmts[i] == hope) {
			return i;
		}
     }
	return -1;
}

AVStream *get_nth_stream(AVFormatContext *fmt_ctx, uint i) {
	return fmt_ctx->streams[i];
}
*/
import "C"

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"unsafe"
)

func Say() {
	s := C.CString("Hello World!\n")
	defer C.free(unsafe.Pointer(s))
	C.puts(s)
}

func GetMetadata(path string) (map[string]string, error) {
	res := make(map[string]string)

	var avctx *C.AVFormatContext = nil

	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	err := avop(C.avformat_open_input(&avctx, cpath, nil, nil))
	if err != nil {
		return res, err
	}
	defer C.avformat_close_input(&avctx)

	blank := C.CString("")
	defer C.free(unsafe.Pointer(blank))

	var tag *C.AVDictionaryEntry = nil
	for true {
		tag = C.av_dict_get(avctx.metadata, blank, tag, C.AV_DICT_IGNORE_SUFFIX)
		if tag == nil {
			break
		}
		res[C.GoString(tag.key)] = C.GoString(tag.value)
	}

	return res, nil
}

func CreateEncoderWEBP(dctx *C.AVCodecContext, pathOut string) (*C.AVFormatContext, *C.AVCodecContext, error) {
	var octx *C.AVFormatContext = nil
	var ectx *C.AVCodecContext = nil

	cfmt := C.CString("webp")
	defer C.free(unsafe.Pointer(cfmt))

	err := avop(C.avformat_alloc_output_context2(&octx, nil, cfmt, nil))
	if err != nil {
		return nil, nil, err
	}

	os := C.avformat_new_stream(octx, nil)
	if os == nil {
		C.avformat_free_context(octx)
		return nil, nil, errors.New("Failed to create stream")
	}

	enc := C.avcodec_find_encoder(C.AV_CODEC_ID_WEBP)
	if enc == nil {
		C.avformat_free_context(octx)
		return nil, nil, errors.New("Failed to find WEBP encoder!")
	}

	ectx = C.avcodec_alloc_context3(enc)
	if ectx == nil {
		C.avformat_free_context(octx)
		return nil, nil, errors.New("Failed to create encoder context")
	}

	h := C.double(540.0)
	ratio := h / (C.double)(dctx.height)
	w := (C.double)(dctx.width) * ratio

	ectx.width = (C.int)(w)
	ectx.height = (C.int)(h)

	pixi := C.get_pix_fmt(enc.pix_fmts, C.AV_PIX_FMT_YUV420P)
	if pixi == -1 {
		C.avcodec_free_context(&ectx)
		C.avformat_free_context(octx)
		return nil, nil, errors.New("Failed to find preferred pixel format")
	}

	ectx.pix_fmt = C.AV_PIX_FMT_YUV420P

	ectx.time_base.num = 1
	ectx.time_base.den = 1

	err = avop(C.avcodec_open2(ectx, enc, nil))
	if err != nil {
		C.avcodec_free_context(&ectx)
		C.avformat_free_context(octx)
		return nil, nil, err
	}

	err = avop(C.avcodec_parameters_from_context(os.codecpar, ectx))
	if err != nil {
		C.avcodec_free_context(&ectx)
		C.avformat_free_context(octx)
		return nil, nil, err
	}

	pathTmp := C.CString(pathOut)
	defer C.free(unsafe.Pointer(pathTmp))
	err = avop(C.avio_open(&octx.pb, pathTmp, C.AVIO_FLAG_WRITE))
	if err != nil {
		C.avcodec_free_context(&ectx)
		C.avformat_free_context(octx)
		return nil, nil, err
	}

	err = avop(C.avformat_write_header(octx, nil))
	if err != nil {
		C.avcodec_free_context(&ectx)
		C.avformat_free_context(octx)
		return nil, nil, err
	}

	return octx, ectx, nil
}

func CreateEncoderJPG(dctx *C.AVCodecContext, pathOut string) (*C.AVFormatContext, *C.AVCodecContext, error) {
	var octx *C.AVFormatContext = nil
	var ectx *C.AVCodecContext = nil

	cfmt := C.CString("mjpeg")
	defer C.free(unsafe.Pointer(cfmt))

	err := avop(C.avformat_alloc_output_context2(&octx, nil, cfmt, nil))
	if err != nil {
		return nil, nil, err
	}

	os := C.avformat_new_stream(octx, nil)
	if os == nil {
		C.avformat_free_context(octx)
		return nil, nil, errors.New("Failed to create stream")
	}

	enc := C.avcodec_find_encoder(C.AV_CODEC_ID_MJPEG)
	if enc == nil {
		C.avformat_free_context(octx)
		return nil, nil, errors.New("Failed to find JPEG encoder!")
	}

	ectx = C.avcodec_alloc_context3(enc)
	if ectx == nil {
		C.avformat_free_context(octx)
		return nil, nil, errors.New("Failed to create encoder context")
	}

	h := C.double(360.0)
	ratio := h / (C.double)(dctx.height)
	w := (C.double)(dctx.width) * ratio

	ectx.width = (C.int)(w)
	ectx.height = (C.int)(h)

	pixi := C.get_pix_fmt(enc.pix_fmts, C.AV_PIX_FMT_YUVJ420P)
	if pixi == -1 {
		C.avcodec_free_context(&ectx)
		C.avformat_free_context(octx)
		return nil, nil, errors.New("Failed to find preferred pixel format")
	}

	ectx.pix_fmt = C.AV_PIX_FMT_YUVJ420P

	ectx.time_base.num = 1
	ectx.time_base.den = 1

	err = avop(C.avcodec_open2(ectx, enc, nil))
	if err != nil {
		C.avcodec_free_context(&ectx)
		C.avformat_free_context(octx)
		return nil, nil, err
	}

	err = avop(C.avcodec_parameters_from_context(os.codecpar, ectx))
	if err != nil {
		C.avcodec_free_context(&ectx)
		C.avformat_free_context(octx)
		return nil, nil, err
	}

	pathTmp := C.CString(pathOut)
	defer C.free(unsafe.Pointer(pathTmp))
	err = avop(C.avio_open(&octx.pb, pathTmp, C.AVIO_FLAG_WRITE))
	if err != nil {
		C.avcodec_free_context(&ectx)
		C.avformat_free_context(octx)
		return nil, nil, err
	}

	err = avop(C.avformat_write_header(octx, nil))
	if err != nil {
		C.avcodec_free_context(&ectx)
		C.avformat_free_context(octx)
		return nil, nil, err
	}

	return octx, ectx, nil
}

func InitFilters(ctxEnc, ctxDec *C.AVCodecContext) (*C.AVFilterGraph, *C.AVFilterContext, *C.AVFilterContext, error) {
	nameSrc := C.CString("buffer")
	defer C.free(unsafe.Pointer(nameSrc))

	nameSnk := C.CString("buffersink")
	defer C.free(unsafe.Pointer(nameSnk))

	src := C.avfilter_get_by_name(nameSrc)
	snk := C.avfilter_get_by_name(nameSnk)

	fos := C.avfilter_inout_alloc()
	fis := C.avfilter_inout_alloc()

	var ctxSrc *C.AVFilterContext = nil
	var ctxSnk *C.AVFilterContext = nil

	graph := C.avfilter_graph_alloc()

	canvas := fmt.Sprintf("video_size=%dx%d:pix_fmt=%d:time_base=30001/1:pixel_aspect=1/1",
		ctxDec.width, ctxDec.height, ctxDec.pix_fmt)
	//log.Printf("%s\n", canvas)

	canvasArg := C.CString(canvas)
	defer C.free(unsafe.Pointer(canvasArg))

	argIn := C.CString("in")
	defer C.free(unsafe.Pointer(argIn))

	argOut := C.CString("out")
	defer C.free(unsafe.Pointer(argOut))

	err := avop(C.avfilter_graph_create_filter(&ctxSrc, src, argIn, canvasArg, nil, graph))
	if err != nil {
		return nil, nil, nil, err
	}

	err = avop(C.avfilter_graph_create_filter(&ctxSnk, snk, argOut, nil, nil, graph))
	if err != nil {
		return nil, nil, nil, err
	}

	argFmts := C.CString("pix_fmts")
	defer C.free(unsafe.Pointer(argFmts))

	err = avop(C.av_opt_set_bin(
		unsafe.Pointer(ctxSnk), argFmts,
		(*C.uint8_t)(unsafe.Pointer(&ctxEnc.pix_fmt)), C.sizeof_int,
		C.AV_OPT_SEARCH_CHILDREN))
	if err != nil {
		return nil, nil, nil, err
	}

	fos.name = C.av_strdup(argIn)
	fos.filter_ctx = ctxSrc
	fos.pad_idx = 0
	fos.next = nil

	fis.name = C.av_strdup(argOut)
	fis.filter_ctx = ctxSnk
	fis.pad_idx = 0
	fis.next = nil

	spec := fmt.Sprintf("scale=h=%d:w=-1", ctxEnc.height)
	specArg := C.CString(spec)
	defer C.free(unsafe.Pointer(specArg))

	err = avop(C.avfilter_graph_parse_ptr(graph, specArg, &fis, &fos, nil))
	if err != nil {
		return nil, nil, nil, err
	}

	err = avop(C.avfilter_graph_config(graph, nil))
	if err != nil {
		return nil, nil, nil, err
	}

	return graph, ctxSrc, ctxSnk, nil
}

func OpenVideoStream(ctxFmt *C.AVFormatContext) (C.uint, *C.AVCodecContext, error) {
	streamFound := false
	idxStream := C.uint(0)
	for i := C.uint(0); i < ctxFmt.nb_streams; i++ {
		if C.get_nth_stream(ctxFmt, i).codecpar.codec_type == C.AVMEDIA_TYPE_VIDEO {
			streamFound = true
			idxStream = i
			break
		}
	}

	if !streamFound {
		return idxStream, nil, errors.New("No video stream found")
	}

	stream := C.get_nth_stream(ctxFmt, idxStream)
	decoder := C.avcodec_find_decoder(stream.codecpar.codec_id)
	if decoder == nil {
		return idxStream, nil, errors.New("Could not find decoder")
	}

	ctxDec := C.avcodec_alloc_context3(decoder)
	err := avop(C.avcodec_parameters_to_context(ctxDec, stream.codecpar))
	if err != nil {
		return idxStream, nil, err
	}

	err = avop(C.avcodec_open2(ctxDec, decoder, nil))
	if err != nil {
		return idxStream, nil, err
	}

	return idxStream, ctxDec, nil
}

var errSeekFailed = errors.New("Seek failed")

func CreateThumbnail(pathIn string) ([]byte, error) {
	b, err := CreateThumbnailX(pathIn, true)

	// Some streams don't support seeking
	// In this case just do a thumbnail of the first frame
	// Better than nothing... I guess
	if errors.Is(err, errSeekFailed) {
		b, err = CreateThumbnailX(pathIn, false)
	}
	return b, err
}

func CreateThumbnailX(pathIn string, seek bool) ([]byte, error) {
	//return nil, errors.New("This is a test")

	var ctxFmtIn *C.AVFormatContext = nil
	pathInArg := C.CString(pathIn)
	defer C.free(unsafe.Pointer(pathInArg))

	err := avop(C.avformat_open_input(&ctxFmtIn, pathInArg, nil, nil))
	if err != nil {
		// if it fails here, it's because the file wasn't a media file
		return nil, err
	}
	defer C.avformat_close_input(&ctxFmtIn)

	err = avop(C.avformat_find_stream_info(ctxFmtIn, nil))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return nil, err
	}

	idxStream, ctxDec, err := OpenVideoStream(ctxFmtIn)
	if err != nil {
		return nil, err
	}
	defer C.avcodec_close(ctxDec)

	tmpFile, err := os.CreateTemp(os.TempDir(), "servemediago-")
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return nil, err
	}
	defer os.Remove(tmpFile.Name())

	ctxFmtOut, ctxEnc, err := CreateEncoderWEBP(ctxDec, tmpFile.Name())
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return nil, err
	}
	defer C.avformat_free_context(ctxFmtOut)
	defer C.avcodec_close(ctxEnc)
	defer C.avio_closep(&ctxFmtOut.pb)

	graph, ctxSrc, ctxSnk, err := InitFilters(ctxEnc, ctxDec)
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return nil, err
	}
	defer C.avfilter_graph_free(&graph)

	if seek {
		durationSeconds := ctxFmtIn.duration / C.AV_TIME_BASE
		var midPos C.AVRational
		midPos.num = C.int(durationSeconds / 2)
		midPos.den = 1

		rts := C.av_mul_q(midPos, C.av_inv_q(C.get_nth_stream(ctxFmtIn, idxStream).time_base))
		timestamp := C.av_q2d(rts)
		err = avop(C.av_seek_frame(ctxFmtIn, C.int(idxStream), C.long(timestamp), 0))
		if err != nil {
			return nil, errSeekFailed
		}
	}

	pktDec := C.av_packet_alloc()
	defer C.av_packet_free(&pktDec)

	pktEnc := C.av_packet_alloc()
	defer C.av_packet_free(&pktEnc)

	frame := C.av_frame_alloc()
	defer C.av_frame_free(&frame)

	frameFiltered := C.av_frame_alloc()
	defer C.av_frame_free(&frameFiltered)

	for true {
		err = avop(C.av_read_frame(ctxFmtIn, pktDec))
		if err != nil {
			log.Printf("%s: %s\n", pathIn, err)
			return nil, err
		}

		if pktDec.stream_index != C.int(idxStream) {
			C.av_packet_unref(pktDec)
			continue
		}

		err = avop(C.avcodec_send_packet(ctxDec, pktDec))
		if err != nil {
			C.av_packet_unref(pktDec)
			log.Printf("%s: %s\n", pathIn, err)
			return nil, err
		}

		rc := C.avcodec_receive_frame(ctxDec, frame)
		// should be C.AVERROR(C.EAGAIN) but doesn't work
		// AVERROR definition in error.h is
		// #define AVERROR(e) (-(e))
		// so just negative of EAGAIN to check
		if rc == -C.EAGAIN {
			C.av_packet_unref(pktDec)
			continue
		}
		if rc < 0 {
			C.av_packet_unref(pktDec)
			log.Printf("%s: %s\n", pathIn, err)
			return nil, errors.New("Failed to decode frame")
		}

		if frame.pict_type != C.AV_PICTURE_TYPE_I {
			C.av_frame_unref(frame)
			continue
		}

		break
	}

	err = avop(C.av_buffersrc_add_frame_flags(ctxSrc, frame, 0))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return nil, err
	}

	err = avop(C.av_buffersink_get_frame(ctxSnk, frameFiltered))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return nil, err
	}

	err = avop(C.avcodec_send_frame(ctxEnc, frameFiltered))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return nil, err
	}

	err = avop(C.avcodec_send_frame(ctxEnc, nil))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return nil, err
	}

	err = avop(C.avcodec_receive_packet(ctxEnc, pktEnc))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return nil, err
	}

	err = avop(C.av_write_frame(ctxFmtOut, pktEnc))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return nil, err
	}

	C.av_frame_unref(frameFiltered)
	C.av_packet_unref(pktEnc)

	C.av_frame_unref(frame)
	C.av_packet_unref(pktDec)

	err = avop(C.av_write_trailer(ctxFmtOut))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return nil, err
	}

	rawThumb, err := io.ReadAll(tmpFile)
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return nil, err
	}

	return rawThumb, nil
}

func avop(rc C.int) error {
	if rc >= 0 {
		return nil
	}

	errbuf_len := C.ulong(256)
	errbuf := (*C.char)(C.malloc(errbuf_len))
	defer C.free(unsafe.Pointer(errbuf))

	C.av_make_error_string(errbuf, errbuf_len, rc)
	return errors.New(C.GoString(errbuf))
}

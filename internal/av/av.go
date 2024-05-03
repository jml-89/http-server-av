// This was an experiment
// Instead of the usual approach of writing a simple wrapper
// Write functions that program needs that call C API directly
// The verdict: Not much better, really -- just use a wrapper

package av

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

var errNotMediaFile = errors.New("File is not an image or video")
func parseMediaFile(filename string) ([]byte, map[string]string, error) {
	info, err := os.Stat(filename)
	if err != nil {
		log.Println(err)
		return nil, nil, err
	}

	metadata, err := GetMetadata(filename)
	if err != nil {
		if fmt.Sprintf("%s", err) == "Invalid data found when processing input" {
			log.Printf("%s is not a media file", filename)
			// Just isn't a media file
			return nil, nil, errNotMediaFile
		}
		log.Printf("%s: %s", filename, err)
		return nil, nil, err
	}

	thumbnail, err := CreateThumbnail(filename)
	if err != nil {
		log.Printf("Failed to generate thumbnail for %s", filename)
		//return nil, nil, err
	}

	metadata["favourite"] = "false"
	metadata["diskfiletime"] = info.ModTime().UTC().Format("2006-01-02T15:04:05")
	metadata["diskfilename"] = filename
	metadata["diskfilesize"] = fmt.Sprintf("%099d", info.Size())
	thumbname := fmt.Sprintf("%s.webp", filename)
	metadata["thumbname"] = thumbname

	return thumbnail, metadata, nil
}

func GetMetadata(path string) (map[string]string, error) {
	res := make(map[string]string)

	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	var avctx *C.AVFormatContext = nil
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

	for i := C.uint(0); i < avctx.nb_streams; i++ {
		s := C.get_nth_stream(avctx, i)
		if s.codecpar.codec_type == C.AVMEDIA_TYPE_VIDEO {
			res["mediatype"] = "video"
			break
		}

		if s.codecpar.codec_type == C.AVMEDIA_TYPE_AUDIO {
			res["mediatype"] = "audio"
		}
	}

	ts := avctx.duration / C.AV_TIME_BASE
	if ts > 0 {
		tm := ts / 60
		th := tm / 60
		td := th / 24
		res["duration"] = fmt.Sprintf("%02d:%02d:%02d:%02d", td, th%24, tm%60, ts%60)
	} else if res["mediatype"] == "video" {
		res["mediatype"] = "image"
	}

	return res, nil
}

func CreateEncoderWEBP(width, height int, pathOut string) (*C.AVFormatContext, *C.AVCodecContext, error) {
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

	ectx.width = (C.int)(width)
	ectx.height = (C.int)(height)

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

// Adds a filter to a graph and (somewhat) hides the C string management issue
func createFilter(id, filter, args string, graph *C.AVFilterGraph) (*C.AVFilterContext, error) {
	filterC := C.CString(filter)
	defer C.free(unsafe.Pointer(filterC))

	nameC := C.CString(id)
	defer C.free(unsafe.Pointer(nameC))

	argsC := C.CString(args)
	defer C.free(unsafe.Pointer(argsC))

	f := C.avfilter_get_by_name(filterC)
	if f == nil {
		return nil, errors.New(fmt.Sprintf("Filter '%s' not found", filter))
	}

	var ctx *C.AVFilterContext = nil
	err := avop(C.avfilter_graph_create_filter(&ctx, f, nameC, argsC, nil, graph))
	if err != nil {
		return ctx, err
	}

	return ctx, err
}

func InitFiltersTestImage(imgW, imgH int) (*C.AVFilterGraph, *C.AVFilterContext, error) {
	graph := C.avfilter_graph_alloc()

	ctxSrc, err := createFilter("in", "pal75bars", fmt.Sprintf("size=%dx%d", imgW, imgH), graph)
	if err != nil {
		log.Printf("%s\n", err)
		return nil, nil, err
	}

	ctxSnk, err := createFilter("out", "buffersink", "", graph)
	if err != nil {
		log.Printf("%s\n", err)
		return nil, nil, err
	}

	err = avop(C.avfilter_link(ctxSrc, 0, ctxSnk, 0))
	if err != nil {
		log.Printf("%s\n", err)
		return nil, nil, err
	}

	err = avop(C.avfilter_graph_config(graph, nil))
	if err != nil {
		log.Printf("%s\n", err)
		return nil, nil, err
	}

	return graph, ctxSnk, nil
}

func InitFiltersScaling(ctxEnc, ctxDec *C.AVCodecContext) (*C.AVFilterGraph, *C.AVFilterContext, *C.AVFilterContext, error) {
	graph := C.avfilter_graph_alloc()
	ctxSrc, err := createFilter(
		"in",
		"buffer",
		fmt.Sprintf("video_size=%dx%d:pix_fmt=%d:time_base=30001/1:pixel_aspect=1/1",
			ctxDec.width, ctxDec.height, ctxDec.pix_fmt),
		graph)
	ctxSnk, err := createFilter(
		"out",
		"buffersink",
		"",
		graph)
	ctxScale, err := createFilter(
		"scale",
		"scale",
		fmt.Sprintf("h=%d:w=-1", ctxEnc.height),
		graph)

	err = avop(C.avfilter_link(ctxSrc, 0, ctxScale, 0))
	if err != nil {
		log.Printf("%s\n", err)
		return nil, nil, nil, err
	}

	err = avop(C.avfilter_link(ctxScale, 0, ctxSnk, 0))
	if err != nil {
		log.Printf("%s\n", err)
		return nil, nil, nil, err
	}

	argFmts := C.CString("pix_fmts")
	defer C.free(unsafe.Pointer(argFmts))

	err = avop(C.av_opt_set_bin(
		unsafe.Pointer(ctxSnk), argFmts,
		(*C.uint8_t)(unsafe.Pointer(&ctxEnc.pix_fmt)), C.sizeof_int,
		C.AV_OPT_SEARCH_CHILDREN))
	if err != nil {
		log.Println(err)
		return nil, nil, nil, err
	}

	err = avop(C.avfilter_graph_config(graph, nil))
	if err != nil {
		log.Println(err)
		return nil, nil, nil, err
	}

	return graph, ctxSrc, ctxSnk, nil
}

func OpenBestStream(ctxFmt *C.AVFormatContext, avtype int32) (C.uint, *C.AVCodecContext, error) {
	var decoder *C.AVCodec = nil
	is := C.av_find_best_stream(ctxFmt, avtype, -1, -1, &decoder, 0)
	if is < 0 {
		return 0, nil, errors.New("No best stream found")
	}
	idxStream := C.uint(is)

	stream := C.get_nth_stream(ctxFmt, idxStream)

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
	tmpFile, err := os.CreateTemp(os.TempDir(), "httpfileserve-")
	if err != nil {
		log.Printf("%s", err)
		return nil, err
	}
	defer os.Remove(tmpFile.Name())

	err = CreateThumbnailX(pathIn, tmpFile.Name(), true)

	// Some streams don't support seeking
	// In this case just do a thumbnail of the first frame
	// Better than nothing
	if errors.Is(err, errSeekFailed) || fmt.Sprintf("%s", err) == "End of file" {
		err = CreateThumbnailX(pathIn, tmpFile.Name(), false)
		if err != nil {
			log.Printf("%s: %s", pathIn, err)
		}
	}

	// When all else fails, go generic
	if err != nil {
		err = CreateGenericThumbnail(tmpFile.Name())
		if err != nil {
			log.Printf("%s: %s", pathIn, err)
			return nil, err
		}
	}

	b, err := io.ReadAll(tmpFile)
	if err != nil {
		log.Printf("%s", err)
		return nil, err
	}

	return b, err
}

// Just creates a 960x540 test image
// Wanted to do a spectrum picture for audio files, but the filter consumed a lot of memory
// So this is the fallback
func CreateGenericThumbnail(pathOut string) error {
	imgH := 540
	imgW := 960

	ctxFmtOut, ctxEnc, err := CreateEncoderWEBP(imgW, imgH, pathOut)
	if err != nil {
		log.Printf("%s", err)
		return err
	}
	defer C.avformat_free_context(ctxFmtOut)
	defer C.avcodec_free_context(&ctxEnc)
	defer C.avio_closep(&ctxFmtOut.pb)

	graph, ctxSnk, err := InitFiltersTestImage(imgW, imgH)
	if err != nil {
		log.Printf("%s", err)
		return err
	}
	defer C.avfilter_graph_free(&graph)

	pktEnc := C.av_packet_alloc()
	defer C.av_packet_free(&pktEnc)

	frameFiltered := C.av_frame_alloc()
	defer C.av_frame_free(&frameFiltered)

	err = avop(C.av_buffersink_get_frame(ctxSnk, frameFiltered))
	if err != nil {
		log.Printf("%s", err)
		return err
	}

	err = avop(C.avcodec_send_frame(ctxEnc, frameFiltered))
	if err != nil {
		log.Printf("%s", err)
		return err
	}

	err = avop(C.avcodec_send_frame(ctxEnc, nil))
	if err != nil {
		log.Printf("%s", err)
		return err
	}

	err = avop(C.avcodec_receive_packet(ctxEnc, pktEnc))
	if err != nil {
		log.Printf("%s", err)
		return err
	}

	err = avop(C.av_write_frame(ctxFmtOut, pktEnc))
	if err != nil {
		log.Printf("%s", err)
		return err
	}

	C.av_frame_unref(frameFiltered)
	C.av_packet_unref(pktEnc)

	err = avop(C.av_write_trailer(ctxFmtOut))
	if err != nil {
		log.Printf("%s", err)
		return err
	}

	return nil
}

func CreateThumbnailX(pathIn, pathOut string, seek bool) error {
	var ctxFmtIn *C.AVFormatContext = nil
	pathInArg := C.CString(pathIn)
	defer C.free(unsafe.Pointer(pathInArg))

	err := avop(C.avformat_open_input(&ctxFmtIn, pathInArg, nil, nil))
	if err != nil {
		// if it fails here, it's because the file wasn't a media file
		return err
	}
	defer C.avformat_close_input(&ctxFmtIn)

	err = avop(C.avformat_find_stream_info(ctxFmtIn, nil))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return err
	}

	idxStream, ctxDec, err := OpenBestStream(ctxFmtIn, C.AVMEDIA_TYPE_VIDEO)
	if err != nil {
		return err
	}
	defer C.avcodec_free_context(&ctxDec)

	imgH := 540
	ratio := float64(imgH) / float64(ctxDec.height)
	imgW := int(float64(ctxDec.width) * ratio)

	ctxFmtOut, ctxEnc, err := CreateEncoderWEBP(imgW, imgH, pathOut)
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return err
	}
	defer C.avformat_free_context(ctxFmtOut)
	defer C.avcodec_free_context(&ctxEnc)
	defer C.avio_closep(&ctxFmtOut.pb)

	graph, ctxSrc, ctxSnk, err := InitFiltersScaling(ctxEnc, ctxDec)
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return err
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
			return errSeekFailed
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
			return err
		}
		defer C.av_packet_unref(pktDec)

		if pktDec.stream_index != C.int(idxStream) {
			continue
		}

		err = avop(C.avcodec_send_packet(ctxDec, pktDec))
		if err != nil {
			log.Printf("%s: %s\n", pathIn, err)
			return err
		}

		rc := C.avcodec_receive_frame(ctxDec, frame)
		// should be C.AVERROR(C.EAGAIN) but doesn't work
		// AVERROR definition in error.h is
		// #define AVERROR(e) (-(e))
		// so just negative of EAGAIN to check
		if rc == -C.EAGAIN {
			continue
		}
		if rc < 0 {
			log.Printf("%s: %s\n", pathIn, err)
			return errors.New("Failed to decode frame")
		}
		defer C.av_frame_unref(frame)

		if frame.pict_type != C.AV_PICTURE_TYPE_I {
			continue
		}

		err = avop(C.av_buffersrc_add_frame_flags(ctxSrc, frame, 0))
		if err != nil {
			log.Printf("%s: %s\n", pathIn, err)
			return err
		}

		break
	}

	err = avop(C.av_buffersink_get_frame(ctxSnk, frameFiltered))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return err
	}

	err = avop(C.avcodec_send_frame(ctxEnc, frameFiltered))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return err
	}

	err = avop(C.avcodec_send_frame(ctxEnc, nil))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return err
	}

	err = avop(C.avcodec_receive_packet(ctxEnc, pktEnc))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return err
	}

	err = avop(C.av_write_frame(ctxFmtOut, pktEnc))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return err
	}

	C.av_frame_unref(frameFiltered)
	C.av_packet_unref(pktEnc)

	C.av_frame_unref(frame)
	C.av_packet_unref(pktDec)

	err = avop(C.av_write_trailer(ctxFmtOut))
	if err != nil {
		log.Printf("%s: %s\n", pathIn, err)
		return err
	}

	return nil
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

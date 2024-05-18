#pragma once 

// Just going to have these guys defined in the header file
// If it was a larger file, then, well

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


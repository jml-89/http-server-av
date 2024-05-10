#include "thumbnailer.h"

#include <vector>
#include <iostream>

#include <opencv2/core.hpp>
#include <opencv2/imgcodecs.hpp>
#include <opencv2/imgproc.hpp>

#include <opencv2/videoio.hpp> 

#include <ranges>

#include "util.hpp"

thumbnailer *thumbnailer_init(char *path_model_detect, char *path_model_assess) {
	auto one = std::string(path_model_detect);
	auto two = std::string(path_model_assess);
	return new thumbnailer(one, two);
}

void thumbnailer_free(thumbnailer *t) {
	delete t;
}

void thumbnailer_run(thumbnailer *t, char *path_in, char *path_out) {
	auto one = std::string(path_in);
	auto two = std::string(path_out);
	t->run(one, two);
}

struct candidate {
	cv::Mat image;
	float quality;
	float confidence;
	int frame_pos;
	int area;
};

thumbnailer::thumbnailer(const std::string& path_detect, const std::string& path_assess) :
	face_finder(yolo(path_detect, path_assess))
{}

void thumbnailer::run(const std::string& path_video, const std::string& path_out) {
	auto cap = cv::VideoCapture(path_video, cv::CAP_FFMPEG);

	std::vector<candidate> candidates;

	auto frame_count = static_cast<int>(cap.get(cv::CAP_PROP_FRAME_COUNT));
	auto probes = 8;
	auto stride = frame_count / probes;
	auto offset = stride / 2;
	for (int i = 0; i < probes; i++) {
		auto frame_pos = offset + (i * stride);
		cap.set(cv::CAP_PROP_POS_FRAMES, static_cast<double>(frame_pos));

		cv::Mat frame;
		if (!cap.read(frame) || frame.empty()) {
			break;
		}

		auto finds = face_finder.find(frame);
		if (finds.size() == 0) {
			continue;
		}

		candidate x;
		x.image = frame;
		x.frame_pos = frame_pos;
		x.quality = 0.0f;
		x.confidence = 0.0f;
		x.area = 0;

		for (auto find : finds) {
			if (find.quality > x.quality) {
				x.quality = find.quality;
			}

			if (find.confidence > x.confidence) {
				x.confidence = find.confidence;
			}
			auto area = find.box_scaled.width * find.box_scaled.height;
			x.area += area;
		}

		if (x.area > 10000) {
			candidates.push_back(x);
		}
	}

	if (candidates.size() < 1) {
		return;
	}

	std::ranges::sort(candidates, {}, [](const auto& c) { 
		return c.confidence * std::min(c.quality, 0.7f);
	});
	std::ranges::reverse(candidates);

	int count = 0;
	for (const auto& candidate : candidates) {
		count++;
		if (count > 1) { break; }
		cv::imwrite(path_out, image_scale(candidate.image, 960, 540));
	}
}


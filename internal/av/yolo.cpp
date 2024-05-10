#include "yolo.hpp"
#include <opencv2/imgproc.hpp>
#include <algorithm>

#include "util.hpp"

#include <iostream>
#include <ranges>

yolo_face::yolo_face(
	std::string path_model, 
	float confidence_threshold, 
	float nms_threshold
) : 
	confidence_threshold(confidence_threshold), 
	nms_threshold(nms_threshold) 
{
	net = cv::dnn::readNet(path_model);
}

std::vector<proposal> yolo_face::nms_filter(const std::vector<proposal>& proposals) {
	// Just mucking around with features here
	std::vector<float> confidences;
	std::ranges::transform(
		proposals,
		std::back_inserter(confidences), 
		[](const auto& p) { return p.confidence; }
	);

	// std::views syntax is crazy
	std::vector<cv::Rect> boxes;
	std::ranges::copy(
		proposals | std::views::transform([] (const auto& p) { return p.box_scaled; }), 
		std::back_inserter(boxes)
	);

	std::vector<int> indices;
	cv::dnn::NMSBoxes(
		boxes, confidences,
		confidence_threshold,
		nms_threshold,
		indices
	);

	std::vector<proposal> filtered;
	for (auto index : indices) {
		auto area = proposals[index].box_scaled.width * proposals[index].box_scaled.height;
		if (area > (30*30)) {
			filtered.push_back(proposals[index]);
		}
	}

	return filtered;
}

cv::Rect2i scale_box(cv::Rect2f box, cv::Mat image, cv::Mat output) {
	auto stride = static_cast<int>(ceil(static_cast<float>(image.rows) / output.size[2]));
	cv::Rect2i ret;
	ret.x = box.x * stride;
	ret.y = box.y * stride;
	ret.width = box.width * stride;
	ret.height = box.height * stride;
	return ret;
}

std::vector<proposal> yolo_face::detect(cv::Mat image) {
	image = image_pad_square(image_scale(image, width, height));

	auto blob = cv::dnn::blobFromImage(
		image, 
		1/255.0, // scalefactor
		cv::Size(width, height), 
		cv::Scalar(0, 0, 0), // mean
		true, // swapRB (openCV is BGR-based?)
		false // crop
	);
	net.setInput(blob);

	std::vector<cv::Mat> outputs;
	net.forward(outputs, net.getUnconnectedOutLayersNames());

	std::vector<proposal> proposals;
	for (const auto& output : outputs) {
		for (auto& find : generate_proposals(output)) {
			// Proposal bounding boxes are in output image space, e.g. tiny
			// Scale them up
			find.image_work = image;
			find.box_scaled = scale_box(find.box_raw, image, output);
			proposals.push_back(find);
		}
	}

	return nms_filter(proposals);
}

std::vector<proposal> yolo_face::generate_proposals(cv::Mat output) {
	const int feat_h = output.size[2];
	const int feat_w = output.size[3];
	const int area = feat_w * feat_h;

	// point arithmetic...
	auto ptr = reinterpret_cast<float*>(output.data);
	auto ptr_cls = ptr + area * reg_max * 4;
	auto ptr_kp = ptr + area * (reg_max * 4 + 1);

	std::vector<proposal> proposals;

	for (int i = 0; i < feat_h; i++) {
		for (int j = 0; j < feat_w; j++) {
			const auto idx = (i * feat_w) + j;
			auto box_prob = sigmoid_x(ptr_cls[idx]);
			if (box_prob < confidence_threshold) {
				continue;
			}

			std::vector<float> pred_ltrb;
			for (int k = 0; k < 4; k++) {
				std::vector<float> dfl_value;
				for (int n = 0; n < reg_max; n++) {
					auto wtf = ptr[(k*reg_max + n)*area + idx];
					dfl_value.push_back(wtf);
					//std::cout << std::format("dflval {}: {}\n", n, wtf);
				}
				auto dfl_softmax = softmax(dfl_value);

				float dis = 0.f;
				for (int n = 0; n < reg_max; n++) {
					//std::cout << std::format("softmax {}: {}\n", n, dfl_softmax[n]);
					dis += n * dfl_softmax[n];
				}

				//std::cout << std::format("dis {}\n", dis);
				pred_ltrb.push_back(dis);
			}

			float cx = j + 0.5f, cy = i + 0.5f;
			float xmin = std::max(cx-pred_ltrb[0], 0.f);
			float ymin = std::max(cy-pred_ltrb[1], 0.f);
			float xmax = std::min(cx+pred_ltrb[2], static_cast<float>(feat_h));
			float ymax = std::min(cy+pred_ltrb[3], static_cast<float>(feat_w));

			proposal prop;
			prop.confidence = box_prob;
			prop.box_raw = cv::Rect2f(xmin, ymin, xmax-xmin, ymax-ymin);

			for (int k = 0; k < 5; k++) {
				prop.landmarks_raw.push_back(cv::Point(
					static_cast<int>(ptr_kp[(k*3)*area+idx]*2+j),
					static_cast<int>(ptr_kp[(k*3+1)*area+idx]*2+i)
				));
			}

			proposals.push_back(prop);
		}
	}

	return proposals;
}

yolo_qual::yolo_qual(std::string path_model) {
	net = cv::dnn::readNet(path_model);
}

float yolo_qual::assess(cv::Mat image) {
	cv::Mat image_rgb;
	cv::cvtColor(image, image_rgb, cv::COLOR_BGR2RGB);
	//image_rgb = image_scale(image_rgb, width, height);
	cv::resize(image_rgb, image_rgb, cv::Size(width, height));
	cv::Mat normalised = image_normalise(image_rgb);
	cv::Mat blob = cv::dnn::blobFromImage(normalised);

	net.setInput(blob);
	std::vector<cv::Mat> outputs;
	net.forward(outputs, net.getUnconnectedOutLayersNames());

	auto p = reinterpret_cast<float*>(outputs[0].data);
	auto length = outputs[0].size[1];
	auto quality = 0.f;
	for (int i = 0; i < length; i++) {
		quality += p[i];
	}
	quality /= length;
	return quality;
}

cv::Mat yolo_qual::image_normalise(cv::Mat image) {
	std::vector<cv::Mat> channels;
	cv::split(image, channels);
	for (int i = 0; i < 3; i++) {
		channels[i].convertTo(
			channels[i], 
			CV_32FC1, 
			1.0 / (255.0 * std_devs[i]),
			(0.0 - means[i]) / std_devs[i]
		);
	}
	cv::Mat normalised;
	cv::merge(channels, normalised);
	return normalised;
}

yolo::yolo(std::string path_model_detect) :
	use_qual(false),
	face(path_model_detect, 0.60, 0.5)
{}

yolo::yolo(std::string path_model_detect, std::string path_model_assess) :
	use_qual(true),
	face(path_model_detect, 0.60, 0.5),
	qual(path_model_assess)
{}

std::vector<proposal> yolo::find(cv::Mat image) {
	auto finds = face.detect(image);
	if (!use_qual) {
		return finds; 
	}

	for (auto& find : finds) {
		find.image_face = find.image_work(find.box_scaled);
		find.quality = qual.assess(find.image_face);
	}
	return finds;
}


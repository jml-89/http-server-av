#pragma once

#include <vector>

#include <opencv2/core.hpp> //cv::Mat, cv::Rect, cv::Point
#include <opencv2/dnn.hpp> //cv::dnn::Net

struct proposal {
	cv::Mat image_work;
	cv::Mat image_face;

	float confidence;
	float quality;

	cv::Rect2f box_raw;
	cv::Rect2i box_scaled;

	std::vector<cv::Point2f> landmarks_raw;
	std::vector<cv::Point2i> landmarks_scaled;
};

class yolo_face {
public:
	yolo_face() = default;
	yolo_face(const std::string& path_model, float confidence_threshold, float nms_threshold);
	std::vector<proposal> detect(const cv::Mat& image);

private:
	float confidence_threshold;
	float nms_threshold;
	cv::dnn::Net net;

	const int width = 640;
	const int height = 640;
	const int reg_max = 16;

	std::vector<proposal> generate_proposals(const cv::Mat& output);
	std::vector<proposal> nms_filter(const std::vector<proposal>& proposals);
};

class yolo_qual {
public:
	yolo_qual() = default;
	yolo_qual(const std::string& path_model);
	float assess(const cv::Mat& image);

private:
	const int width = 112;
	const int height = 112;
	cv::dnn::Net net;

	const std::vector<float> means = { 0.5, 0.5, 0.5 };
	const std::vector<float> std_devs = { 0.5, 0.5, 0.5 };

	cv::Mat image_normalise(const cv::Mat& image);
};

class yolo {
public: 
	yolo(const std::string& path_model_detect, const std::string& path_model_assess);
	yolo(const std::string& path_model_detect);
	std::vector<proposal> find(const cv::Mat& image);

private:
	bool use_qual = false;
	yolo_face face;
	yolo_qual qual;
};


#include "util.hpp"

float sigmoid_x(float x) {
	return 1.f / (1.f + exp(-x));
}

std::vector<float> softmax(std::vector<float> xs) {
	float sum = 0;
	std::vector<float> ys;
	for (const auto& x : xs) {
		auto y = exp(x);
		ys.emplace_back(y);
		sum += y;
	}

	for (auto& y : ys) {
		y /= sum;
	}

	return ys;
}

cv::Mat image_scale(cv::Mat x, int w, int h) {
	cv::Mat y;
	auto min_ratio = std::min(
		static_cast<float>(w) / static_cast<float>(x.cols), 
		static_cast<float>(h) / static_cast<float>(x.rows)
	);
	cv::resize(x, y, y.size(), min_ratio, min_ratio, cv::INTER_AREA);
	return y;
}

cv::Mat image_pad_square(cv::Mat x) {
	if (x.cols == x.rows) {
		return x;
	}

	cv::Mat y;
	if (x.rows < x.cols) {
		auto diff = x.cols - x.rows;
		auto top = diff / 2;
		auto bottom = diff - top;
		cv::copyMakeBorder(x, y, top, bottom, 0, 0, cv::BORDER_CONSTANT, 0);
	} else {
		auto diff = x.rows - x.cols;
		auto left = diff / 2;
		auto right = diff - left;
		cv::copyMakeBorder(x, y, 0, 0, left, right, cv::BORDER_CONSTANT, 0);
	}

	return y;
}


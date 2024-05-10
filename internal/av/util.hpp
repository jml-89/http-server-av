#pragma once

#include <vector>
#include <opencv2/core.hpp>
#include <opencv2/imgproc.hpp>

float sigmoid_x(float x); 
std::vector<float> softmax(std::vector<float> xs);
cv::Mat image_scale(cv::Mat x, int w, int h);
cv::Mat image_pad_square(cv::Mat x);


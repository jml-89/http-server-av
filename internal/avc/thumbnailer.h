#pragma once

#ifdef __cplusplus
#include "yolo.hpp"
struct face {
	int area;
	float confidence;
	float quality;
};

struct face_ret {
	face faces[16];
	size_t len;
};

class thumbnailer {
public:
	thumbnailer(const std::string& path_model_detect, const std::string& path_model_assess);
	void run(const std::string& path_input, const std::string& path_output, int probes);
	std::vector<face> run_image(const std::string& path_input);
	std::vector<face> run_image_buf(const cv::Mat1b& buf);
private:
	yolo face_finder;
};
#else
typedef struct face_s {
	int area;
	float confidence;
	float quality;
} face;

typedef struct face_ret_s {
	face faces[16];
	size_t len;
} face_ret;

typedef struct thumbnailer_s {
	// nothing!
} thumbnailer;
#endif


#ifdef __cplusplus
extern "C" {
#endif

extern thumbnailer *thumbnailer_init(char *path_model_detect, char *path_model_assess);
extern void thumbnailer_free(thumbnailer*);
extern void thumbnailer_run(thumbnailer *t, char *path_video, char *path_thumb, int probes);
extern face_ret thumbnailer_run_image(thumbnailer *t, char *path_image);
extern face_ret thumbnailer_run_image_buf(thumbnailer *t, unsigned char *buf, size_t len);
extern void cv_set_num_threads(int n);

#ifdef __cplusplus
}
#endif

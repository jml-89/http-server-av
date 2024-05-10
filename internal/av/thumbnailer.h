#pragma once

#ifdef __cplusplus
#include "yolo.hpp"
class thumbnailer {
public:
	thumbnailer(const std::string& path_model_detect, const std::string& path_model_assess);
	void run(const std::string& path_input, const std::string& path_output);
private:
	yolo face_finder;
};
#else
typedef struct thumbnailer_s {
	// nothing!
} thumbnailer;
#endif


#ifdef __cplusplus
extern "C" {
#endif

extern thumbnailer *thumbnailer_init(char *path_model_detect, char *path_model_assess);
extern void thumbnailer_free(thumbnailer*);
extern void thumbnailer_run(thumbnailer *t, char *path_video, char *path_thumb);

#ifdef __cplusplus
}
#endif

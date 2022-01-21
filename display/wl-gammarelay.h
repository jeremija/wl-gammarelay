#include <wayland-client.h>

/* Color setting */
typedef struct {
	int temperature;
	float gamma[3];
	float brightness;
} color_setting_t;

int wl_gammarelay_color_set(struct wl_display *display, color_setting_t setting);

int wl_gammarelay_init(struct wl_display **p_display);

#include <wayland-client.h>
#include "wlr-gamma-control-unstable-v1-client-protocol.h"

/* Color setting */
typedef struct {
	int temperature;
	float gamma[3];
	float brightness;
} color_setting_t;

typedef struct {
	struct wl_registry *registry;

	struct wl_display *display;

	struct wl_list outputs;

	struct zwlr_gamma_control_manager_v1 *gamma_control_manager;
} wl_gammarelay_t;

int wl_gammarelay_color_set(wl_gammarelay_t *state, color_setting_t setting);

wl_gammarelay_t *wl_gammarelay_init();

void wl_gammarelay_destroy(wl_gammarelay_t *state);

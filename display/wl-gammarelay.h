#include <wayland-client.h>
#include "wlr-gamma-control-unstable-v1-client-protocol.h"

/* Color setting */
typedef struct {
	int temperature;
	float gamma[3];
	float brightness;
} color_setting_t;

typedef struct {
	struct wl_display *display;

	struct wl_list outputs;

	struct zwlr_gamma_control_manager_v1 *gamma_control_manager;
} wl_gammarelay_state_t;

int wl_gammarelay_color_set(wl_gammarelay_state_t *state, color_setting_t setting);

int wl_gammarelay_init(wl_gammarelay_state_t *state);

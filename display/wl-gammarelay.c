#include <wayland-client-core.h>
#include <wayland-util.h>
#define _POSIX_C_SOURCE 200809L
#include <errno.h>
#include <poll.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/types.h>
#include <sys/eventfd.h>
#include <fcntl.h>
#include <unistd.h>
#include <wayland-client-protocol.h>
#include <wayland-client.h>
#include "wlr-gamma-control-unstable-v1-client-protocol.h"

#include "wl-gammarelay.h"

static const float blackbody_color[] = {
	1.00000000,  0.18172716,  0.00000000, /* 1000K */
	1.00000000,  0.25503671,  0.00000000, /* 1100K */
	1.00000000,  0.30942099,  0.00000000, /* 1200K */
	1.00000000,  0.35357379,  0.00000000, /* ...   */
	1.00000000,  0.39091524,  0.00000000,
	1.00000000,  0.42322816,  0.00000000,
	1.00000000,  0.45159884,  0.00000000,
	1.00000000,  0.47675916,  0.00000000,
	1.00000000,  0.49923747,  0.00000000,
	1.00000000,  0.51943421,  0.00000000,
	1.00000000,  0.54360078,  0.08679949,
	1.00000000,  0.56618736,  0.14065513,
	1.00000000,  0.58734976,  0.18362641,
	1.00000000,  0.60724493,  0.22137978,
	1.00000000,  0.62600248,  0.25591950,
	1.00000000,  0.64373109,  0.28819679,
	1.00000000,  0.66052319,  0.31873863,
	1.00000000,  0.67645822,  0.34786758,
	1.00000000,  0.69160518,  0.37579588,
	1.00000000,  0.70602449,  0.40267128,
	1.00000000,  0.71976951,  0.42860152,
	1.00000000,  0.73288760,  0.45366838,
	1.00000000,  0.74542112,  0.47793608,
	1.00000000,  0.75740814,  0.50145662,
	1.00000000,  0.76888303,  0.52427322,
	1.00000000,  0.77987699,  0.54642268,
	1.00000000,  0.79041843,  0.56793692,
	1.00000000,  0.80053332,  0.58884417,
	1.00000000,  0.81024551,  0.60916971,
	1.00000000,  0.81957693,  0.62893653,
	1.00000000,  0.82854786,  0.64816570,
	1.00000000,  0.83717703,  0.66687674,
	1.00000000,  0.84548188,  0.68508786,
	1.00000000,  0.85347859,  0.70281616,
	1.00000000,  0.86118227,  0.72007777,
	1.00000000,  0.86860704,  0.73688797,
	1.00000000,  0.87576611,  0.75326132,
	1.00000000,  0.88267187,  0.76921169,
	1.00000000,  0.88933596,  0.78475236,
	1.00000000,  0.89576933,  0.79989606,
	1.00000000,  0.90198230,  0.81465502,
	1.00000000,  0.90963069,  0.82838210,
	1.00000000,  0.91710889,  0.84190889,
	1.00000000,  0.92441842,  0.85523742,
	1.00000000,  0.93156127,  0.86836903,
	1.00000000,  0.93853986,  0.88130458,
	1.00000000,  0.94535695,  0.89404470,
	1.00000000,  0.95201559,  0.90658983,
	1.00000000,  0.95851906,  0.91894041,
	1.00000000,  0.96487079,  0.93109690,
	1.00000000,  0.97107439,  0.94305985,
	1.00000000,  0.97713351,  0.95482993,
	1.00000000,  0.98305189,  0.96640795,
	1.00000000,  0.98883326,  0.97779486,
	1.00000000,  0.99448139,  0.98899179,
	1.00000000,  1.00000000,  1.00000000, /* 6500K */
	0.98947904,  0.99348723,  1.00000000,
	0.97940448,  0.98722715,  1.00000000,
	0.96975025,  0.98120637,  1.00000000,
	0.96049223,  0.97541240,  1.00000000,
	0.95160805,  0.96983355,  1.00000000,
	0.94303638,  0.96443333,  1.00000000,
	0.93480451,  0.95923080,  1.00000000,
	0.92689056,  0.95421394,  1.00000000,
	0.91927697,  0.94937330,  1.00000000,
	0.91194747,  0.94470005,  1.00000000,
	0.90488690,  0.94018594,  1.00000000,
	0.89808115,  0.93582323,  1.00000000,
	0.89151710,  0.93160469,  1.00000000,
	0.88518247,  0.92752354,  1.00000000,
	0.87906581,  0.92357340,  1.00000000,
	0.87315640,  0.91974827,  1.00000000,
	0.86744421,  0.91604254,  1.00000000,
	0.86191983,  0.91245088,  1.00000000,
	0.85657444,  0.90896831,  1.00000000,
	0.85139976,  0.90559011,  1.00000000,
	0.84638799,  0.90231183,  1.00000000,
	0.84153180,  0.89912926,  1.00000000,
	0.83682430,  0.89603843,  1.00000000,
	0.83225897,  0.89303558,  1.00000000,
	0.82782969,  0.89011714,  1.00000000,
	0.82353066,  0.88727974,  1.00000000,
	0.81935641,  0.88452017,  1.00000000,
	0.81530175,  0.88183541,  1.00000000,
	0.81136180,  0.87922257,  1.00000000,
	0.80753191,  0.87667891,  1.00000000,
	0.80380769,  0.87420182,  1.00000000,
	0.80018497,  0.87178882,  1.00000000,
	0.79665980,  0.86943756,  1.00000000,
	0.79322843,  0.86714579,  1.00000000,
	0.78988728,  0.86491137,  1.00000000, /* 10000K */
	0.78663296,  0.86273225,  1.00000000,
	0.78346225,  0.86060650,  1.00000000,
	0.78037207,  0.85853224,  1.00000000,
	0.77735950,  0.85650771,  1.00000000,
	0.77442176,  0.85453121,  1.00000000,
	0.77155617,  0.85260112,  1.00000000,
	0.76876022,  0.85071588,  1.00000000,
	0.76603147,  0.84887402,  1.00000000,
	0.76336762,  0.84707411,  1.00000000,
	0.76076645,  0.84531479,  1.00000000,
	0.75822586,  0.84359476,  1.00000000,
	0.75574383,  0.84191277,  1.00000000,
	0.75331843,  0.84026762,  1.00000000,
	0.75094780,  0.83865816,  1.00000000,
	0.74863017,  0.83708329,  1.00000000,
	0.74636386,  0.83554194,  1.00000000,
	0.74414722,  0.83403311,  1.00000000,
	0.74197871,  0.83255582,  1.00000000,
	0.73985682,  0.83110912,  1.00000000,
	0.73778012,  0.82969211,  1.00000000,
	0.73574723,  0.82830393,  1.00000000,
	0.73375683,  0.82694373,  1.00000000,
	0.73180765,  0.82561071,  1.00000000,
	0.72989845,  0.82430410,  1.00000000,
	0.72802807,  0.82302316,  1.00000000,
	0.72619537,  0.82176715,  1.00000000,
	0.72439927,  0.82053539,  1.00000000,
	0.72263872,  0.81932722,  1.00000000,
	0.72091270,  0.81814197,  1.00000000,
	0.71922025,  0.81697905,  1.00000000,
	0.71756043,  0.81583783,  1.00000000,
	0.71593234,  0.81471775,  1.00000000,
	0.71433510,  0.81361825,  1.00000000,
	0.71276788,  0.81253878,  1.00000000,
	0.71122987,  0.81147883,  1.00000000,
	0.70972029,  0.81043789,  1.00000000,
	0.70823838,  0.80941546,  1.00000000,
	0.70678342,  0.80841109,  1.00000000,
	0.70535469,  0.80742432,  1.00000000,
	0.70395153,  0.80645469,  1.00000000,
	0.70257327,  0.80550180,  1.00000000,
	0.70121928,  0.80456522,  1.00000000,
	0.69988894,  0.80364455,  1.00000000,
	0.69858167,  0.80273941,  1.00000000,
	0.69729688,  0.80184943,  1.00000000,
	0.69603402,  0.80097423,  1.00000000,
	0.69479255,  0.80011347,  1.00000000,
	0.69357196,  0.79926681,  1.00000000,
	0.69237173,  0.79843391,  1.00000000,
	0.69119138,  0.79761446,  1.00000000, /* 15000K */
	0.69003044,  0.79680814,  1.00000000,
	0.68888844,  0.79601466,  1.00000000,
	0.68776494,  0.79523371,  1.00000000,
	0.68665951,  0.79446502,  1.00000000,
	0.68557173,  0.79370830,  1.00000000,
	0.68450119,  0.79296330,  1.00000000,
	0.68344751,  0.79222975,  1.00000000,
	0.68241029,  0.79150740,  1.00000000,
	0.68138918,  0.79079600,  1.00000000,
	0.68038380,  0.79009531,  1.00000000,
	0.67939381,  0.78940511,  1.00000000,
	0.67841888,  0.78872517,  1.00000000,
	0.67745866,  0.78805526,  1.00000000,
	0.67651284,  0.78739518,  1.00000000,
	0.67558112,  0.78674472,  1.00000000,
	0.67466317,  0.78610368,  1.00000000,
	0.67375872,  0.78547186,  1.00000000,
	0.67286748,  0.78484907,  1.00000000,
	0.67198916,  0.78423512,  1.00000000,
	0.67112350,  0.78362984,  1.00000000,
	0.67027024,  0.78303305,  1.00000000,
	0.66942911,  0.78244457,  1.00000000,
	0.66859988,  0.78186425,  1.00000000,
	0.66778228,  0.78129191,  1.00000000,
	0.66697610,  0.78072740,  1.00000000,
	0.66618110,  0.78017057,  1.00000000,
	0.66539706,  0.77962127,  1.00000000,
	0.66462376,  0.77907934,  1.00000000,
	0.66386098,  0.77854465,  1.00000000,
	0.66310852,  0.77801705,  1.00000000,
	0.66236618,  0.77749642,  1.00000000,
	0.66163375,  0.77698261,  1.00000000,
	0.66091106,  0.77647551,  1.00000000,
	0.66019791,  0.77597498,  1.00000000,
	0.65949412,  0.77548090,  1.00000000,
	0.65879952,  0.77499315,  1.00000000,
	0.65811392,  0.77451161,  1.00000000,
	0.65743716,  0.77403618,  1.00000000,
	0.65676908,  0.77356673,  1.00000000,
	0.65610952,  0.77310316,  1.00000000,
	0.65545831,  0.77264537,  1.00000000,
	0.65481530,  0.77219324,  1.00000000,
	0.65418036,  0.77174669,  1.00000000,
	0.65355332,  0.77130560,  1.00000000,
	0.65293404,  0.77086988,  1.00000000,
	0.65232240,  0.77043944,  1.00000000,
	0.65171824,  0.77001419,  1.00000000,
	0.65112144,  0.76959404,  1.00000000,
	0.65053187,  0.76917889,  1.00000000,
	0.64994941,  0.76876866,  1.00000000, /* 20000K */
	0.64937392,  0.76836326,  1.00000000,
	0.64880528,  0.76796263,  1.00000000,
	0.64824339,  0.76756666,  1.00000000,
	0.64768812,  0.76717529,  1.00000000,
	0.64713935,  0.76678844,  1.00000000,
	0.64659699,  0.76640603,  1.00000000,
	0.64606092,  0.76602798,  1.00000000,
	0.64553103,  0.76565424,  1.00000000,
	0.64500722,  0.76528472,  1.00000000,
	0.64448939,  0.76491935,  1.00000000,
	0.64397745,  0.76455808,  1.00000000,
	0.64347129,  0.76420082,  1.00000000,
	0.64297081,  0.76384753,  1.00000000,
	0.64247594,  0.76349813,  1.00000000,
	0.64198657,  0.76315256,  1.00000000,
	0.64150261,  0.76281076,  1.00000000,
	0.64102399,  0.76247267,  1.00000000,
	0.64055061,  0.76213824,  1.00000000,
	0.64008239,  0.76180740,  1.00000000,
	0.63961926,  0.76148010,  1.00000000,
	0.63916112,  0.76115628,  1.00000000,
	0.63870790,  0.76083590,  1.00000000,
	0.63825953,  0.76051890,  1.00000000,
	0.63781592,  0.76020522,  1.00000000,
	0.63737701,  0.75989482,  1.00000000,
	0.63694273,  0.75958764,  1.00000000,
	0.63651299,  0.75928365,  1.00000000,
	0.63608774,  0.75898278,  1.00000000,
	0.63566691,  0.75868499,  1.00000000,
	0.63525042,  0.75839025,  1.00000000,
	0.63483822,  0.75809849,  1.00000000,
	0.63443023,  0.75780969,  1.00000000,
	0.63402641,  0.75752379,  1.00000000,
	0.63362667,  0.75724075,  1.00000000,
	0.63323097,  0.75696053,  1.00000000,
	0.63283925,  0.75668310,  1.00000000,
	0.63245144,  0.75640840,  1.00000000,
	0.63206749,  0.75613641,  1.00000000,
	0.63168735,  0.75586707,  1.00000000,
	0.63131096,  0.75560036,  1.00000000,
	0.63093826,  0.75533624,  1.00000000,
	0.63056920,  0.75507467,  1.00000000,
	0.63020374,  0.75481562,  1.00000000,
	0.62984181,  0.75455904,  1.00000000,
	0.62948337,  0.75430491,  1.00000000,
	0.62912838,  0.75405319,  1.00000000,
	0.62877678,  0.75380385,  1.00000000,
	0.62842852,  0.75355685,  1.00000000,
	0.62808356,  0.75331217,  1.00000000,
	0.62774186,  0.75306977,  1.00000000, /* 25000K */
	0.62740336,  0.75282962,  1.00000000  /* 25100K */
};


static void
interpolate_color(float a, const float *c1, const float *c2, float *c)
{
	c[0] = (1.0-a)*c1[0] + a*c2[0];
	c[1] = (1.0-a)*c1[1] + a*c2[1];
	c[2] = (1.0-a)*c1[2] + a*c2[2];
}


/* Helper macro used in the fill functions */
#define F(Y, C)  pow((Y) * setting->brightness * \
		     white_point[C], 1.0/setting->gamma[C])

static void
colorramp_fill(uint16_t *gamma_r, uint16_t *gamma_g, uint16_t *gamma_b,
	       int ramp_size, const color_setting_t *setting)
{
	/* Approximate white point */
	float white_point[3];
	float alpha = (setting->temperature % 100) / 100.0;
	int temp_index = ((setting->temperature - 1000) / 100)*3;

	interpolate_color(alpha, &blackbody_color[temp_index],
			  &blackbody_color[temp_index+3], white_point);

	for (int i = 0; i < ramp_size; i++) {
		gamma_r[i] = F((double)gamma_r[i]/(UINT16_MAX+1), 0) *
			(UINT16_MAX+1);
		gamma_g[i] = F((double)gamma_g[i]/(UINT16_MAX+1), 1) *
			(UINT16_MAX+1);
		gamma_b[i] = F((double)gamma_b[i]/(UINT16_MAX+1), 2) *
			(UINT16_MAX+1);
	}
}

struct output {
	struct wl_output *wl_output;
	struct zwlr_gamma_control_v1 *gamma_control;
	uint32_t name;
	uint32_t ramp_size;
	struct wl_list link;
};

int output_destroy(struct output *output) {
	if (output->gamma_control) {
		zwlr_gamma_control_v1_destroy(output->gamma_control);
		output->gamma_control = NULL;
	}

	if (output->wl_output) {
		wl_output_destroy(output->wl_output);
		output->wl_output = NULL;
	}


	free(output);
}

static int create_anonymous_file(off_t size) {
	char template[] = "/tmp/wlroots-shared-XXXXXX";
	int fd = mkstemp(template);
	if (fd < 0) {
		return -1;
	}

	int ret;
	do {
		errno = 0;
		ret = ftruncate(fd, size);
	} while (errno == EINTR);
	if (ret < 0) {
		close(fd);
		return -1;
	}

	unlink(template);
	return fd;
}

static int create_gamma_table(size_t table_size, uint16_t **table) {
	int fd = create_anonymous_file(table_size);
	if (fd < 0) {
		fprintf(stderr, "failed to create anonymous file\n");
		return -1;
	}

	void *data =
		mmap(NULL, table_size, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
	if (data == MAP_FAILED) {
		fprintf(stderr, "failed to mmap()\n");
		close(fd);
		return -1;
	}

	*table = data;
	return fd;
}

static void gamma_control_handle_gamma_size(void *data,
		struct zwlr_gamma_control_v1 *gamma_control, uint32_t ramp_size) {
	struct output *output = data;
	output->ramp_size = ramp_size;
}

static void gamma_control_handle_failed(void *data,
		struct zwlr_gamma_control_v1 *gamma_control) {
	fprintf(stderr, "failed to set gamma table\n");
}

static const struct zwlr_gamma_control_v1_listener gamma_control_listener = {
	.gamma_size = gamma_control_handle_gamma_size,
	.failed = gamma_control_handle_failed,
};

static void registry_handle_global(void *data, struct wl_registry *registry,
		uint32_t name, const char *interface, uint32_t version) {
	wl_gammarelay_t *state = data;

	/* fprintf(stderr, "registry handle name: %d, interface: %s\n", name, interface); */

	if (strcmp(interface, wl_output_interface.name) == 0) {
		/* fprintf(stderr, "got output name: %d, interface: %s\n", name, interface); */
		struct output *output = calloc(1, sizeof(struct output));
		output->name = name;
		output->wl_output = wl_registry_bind(registry, name,
			&wl_output_interface, 1);
		wl_list_insert(&state->outputs, &output->link);

		output->gamma_control = zwlr_gamma_control_manager_v1_get_gamma_control(
			state->gamma_control_manager, output->wl_output);
		zwlr_gamma_control_v1_add_listener(output->gamma_control,
			&gamma_control_listener, output);

	} else if (strcmp(interface,
			zwlr_gamma_control_manager_v1_interface.name) == 0) {
		state->gamma_control_manager = wl_registry_bind(registry, name,
			&zwlr_gamma_control_manager_v1_interface, 1);


		// For each existing output, register gamma control.
		struct output *output = NULL;

		wl_list_for_each(output, &state->outputs, link) {
			output->gamma_control = zwlr_gamma_control_manager_v1_get_gamma_control(
				state->gamma_control_manager, output->wl_output);
			zwlr_gamma_control_v1_add_listener(output->gamma_control,
				&gamma_control_listener, output);
		}
	}
}

static void registry_handle_global_remove(void *data,
		struct wl_registry *registry, uint32_t name) {
	fprintf(stderr, "registry remove name: %d\n", name);
	wl_gammarelay_t *state = data;
	struct output *output = NULL;

	wl_list_for_each(output, &state->outputs, link) {
		if (output->name == name) {
			fprintf(stderr, "remove output: %d\n", name);

			wl_list_remove(&output->link);

			output_destroy(output);

			break;
		}
	}
}

int wl_gammarelay_num_init_outputs(wl_gammarelay_t *state) {
	struct output *output = NULL;

	int count = 0;

	wl_list_for_each(output, &state->outputs, link) {
		if (output->ramp_size > 0) {
			count++;
		}
	}

	return count;
}

static const struct wl_registry_listener registry_listener = {
	.global = registry_handle_global,
	.global_remove = registry_handle_global_remove,
};

static void fill_gamma_table(uint16_t *table, uint32_t ramp_size,
		const color_setting_t *setting) {
	uint16_t *r = table;
	uint16_t *g = table + ramp_size;
	uint16_t *b = table + 2 * ramp_size;

	/* Initialize gamma ramps to pure state */
	for (uint32_t i = 0; i < ramp_size; i++) {
		uint16_t value = (double)i / ramp_size * (UINT16_MAX+1);
		r[i] = value;
		g[i] = value;
		b[i] = value;
	}

	colorramp_fill(r, g, b, ramp_size, setting);
}

int wl_gammarelay_color_set(wl_gammarelay_t *state, color_setting_t setting) {
	struct output *output = NULL;

	// First process all pending events that might have happened since this
	// method was last called.
	/* wl_display_roundtrip(state->display); */

	// If we had any newly added displays, initialize their gamma control.
	wl_list_for_each(output, &state->outputs, link) {
		if (output->gamma_control != NULL) {
			continue;
		}

		output->gamma_control = zwlr_gamma_control_manager_v1_get_gamma_control(
			state->gamma_control_manager, output->wl_output);
		zwlr_gamma_control_v1_add_listener(output->gamma_control,
			&gamma_control_listener, output);
	}

	// Process all pending events.
	/* wl_display_roundtrip(state->display); */

	if (state->gamma_control_manager == NULL) {
		fprintf(stderr,
			"compositor doesn't support wlr-gamma-control-unstable-v1\n");
		return -1;
	}

	output = NULL;

	wl_list_for_each(output, &state->outputs, link) {
		if (output->ramp_size == 0) {
			fprintf(stderr,
					"output does not have ramp_size set\n");
			continue;
		}

		uint16_t *table = NULL;
		size_t table_size = output->ramp_size * 3 * sizeof(uint16_t);

		int table_fd = create_gamma_table(table_size, &table);
		if (table_fd < 0) {
			fprintf(stderr,
					"failed to create gamma table\n");
			continue;
		}

		fill_gamma_table(table, output->ramp_size,
			&setting);
		zwlr_gamma_control_v1_set_gamma(output->gamma_control,
			table_fd);

		close(table_fd);

		// Clean up resources.
		if (munmap(table, table_size) == -1) {
			fprintf(stderr,
				"failed to munmap\n");
			return -1;
		}
	}

	return 0;
}

wl_gammarelay_t *wl_gammarelay_init() {
	wl_gammarelay_t *state = calloc(1, sizeof(wl_gammarelay_t));

	state->display = wl_display_connect(NULL);
	if (state->display == NULL) {
		free(state);
		fprintf(stderr, "failed to create display\n");
		return NULL;
	}

	state->display_fd = wl_display_get_fd(state->display);

	wl_list_init(&state->outputs);

	state->registry = wl_display_get_registry(state->display);
	wl_registry_add_listener(state->registry, &registry_listener, state);
	wl_display_roundtrip(state->display);

	state->interrupt_fd = eventfd(0, O_CLOEXEC);

	return state;
}

int wl_gammarelay_poll(wl_gammarelay_t *state) {
	struct pollfd pollfds[2];

	pollfds[0].fd = state->display_fd;
	pollfds[0].events = POLLIN;

	pollfds[1].fd = state->interrupt_fd;
	pollfds[1].events = POLLIN;

	int r = poll(pollfds, 2, -1);

	if (pollfds[1].revents & POLLIN) {
		return -999;
	}


	if (r == 0) {
		// Timeout
		return 0;
	}

	if (r < 0) {
		return -1;
	}

	return r;
}

void wl_gammarelay_interrupt(wl_gammarelay_t *state) {
	unsigned char buf[8] = {0};
	memset(buf, 0x88, 8);
	write(state->interrupt_fd, buf, 8);
}

void wl_gammarelay_destroy(wl_gammarelay_t *state) {
	struct output *output = NULL;

	wl_list_for_each(output, &state->outputs, link) {
		output_destroy(output);
	}

	if (state->gamma_control_manager) {
		zwlr_gamma_control_manager_v1_destroy(state->gamma_control_manager);
		state->gamma_control_manager = NULL;
	}

	if (state->registry) {
		wl_registry_destroy(state->registry);
		state->registry = NULL;
	}

	if (state->display) {
		wl_display_disconnect(state->display);
		state->display = NULL;
	}

	close(state->interrupt_fd);

	free(state);
}

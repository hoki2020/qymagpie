//go:build linux && cgo && !nogui

package fonts

/*
#cgo pkg-config: fontconfig
#include <fontconfig/fontconfig.h>

static FcFontSet *pigeonFonts(void) {
    FcPattern *pattern = FcPatternCreate();
    if (!pattern) return NULL;
    FcPatternAddBool(pattern, FC_SCALABLE, FcTrue);
    FcObjectSet *objects = FcObjectSetBuild(FC_FAMILY, FC_STYLE, FC_WEIGHT, FC_SLANT, FC_WIDTH, NULL);
    if (!objects) { FcPatternDestroy(pattern); return NULL; }
    FcFontSet *fonts = FcFontList(NULL, pattern, objects);
    FcObjectSetDestroy(objects);
    FcPatternDestroy(pattern);
    return fonts;
}
static FcPattern *pigeonFontAt(FcFontSet *set, int i) { return set->fonts[i]; }
static const char *pigeonFontString(FcPattern *font, const char *key) {
    FcChar8 *value = NULL;
    return FcPatternGetString(font, key, 0, &value) == FcResultMatch ? (const char *)value : "";
}
static double pigeonFontNumber(FcPattern *font, const char *key, double fallback) {
    FcValue value;
    if (FcPatternGet(font, key, 0, &value) != FcResultMatch) return fallback;
    if (value.type == FcTypeInteger) return value.u.i;
    if (value.type == FcTypeDouble) return value.u.d;
    return fallback;
}
static const char *pigeonFontFamily(FcPattern *f) { return pigeonFontString(f, FC_FAMILY); }
static const char *pigeonFontStyle(FcPattern *f) { return pigeonFontString(f, FC_STYLE); }
static double pigeonFontWeight(FcPattern *f) { return pigeonFontNumber(f, FC_WEIGHT, -1); }
static double pigeonFontSlant(FcPattern *f) { return pigeonFontNumber(f, FC_SLANT, FC_SLANT_ROMAN); }
static double pigeonFontWidth(FcPattern *f) { return pigeonFontNumber(f, FC_WIDTH, 100); }
*/
import "C"

import "errors"

const Available = true

func installed() ([]Face, error) {
	set := C.pigeonFonts()
	if set == nil {
		return nil, errors.New("Fontconfig could not list installed fonts")
	}
	defer C.FcFontSetDestroy(set)
	faces := make([]Face, 0, int(set.nfont))
	for i := C.int(0); i < set.nfont; i++ {
		font := C.pigeonFontAt(set, i)
		// A variable font's range is not a named instance. Its enumerated
		// named instances have numeric weights and are included normally.
		weight := C.pigeonFontWeight(font)
		if weight < 0 {
			continue
		}
		style := "normal"
		switch int(C.pigeonFontSlant(font)) {
		case C.FC_SLANT_ITALIC:
			style = "italic"
		case C.FC_SLANT_OBLIQUE:
			style = "oblique"
		}
		faces = append(faces, Face{
			Family: C.GoString(C.pigeonFontFamily(font)),
			Name:   C.GoString(C.pigeonFontStyle(font)),
			Weight: float64(C.FcWeightToOpenTypeDouble(weight)),
			Style:  style, Stretch: float64(C.pigeonFontWidth(font)),
		})
	}
	return faces, nil
}

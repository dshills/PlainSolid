package iges

// These statuses distinguish OCCT's owned coordinates and one-solid group
// from the standalone PlainSolid form. Model maps are allowed on circular
// curves only and independently proved by the shape decoder.
func igesBoreRimDirectory(first, second [9]int) bool {
	kind, transform, status, form := first[0], first[6], first[8], second[4]
	if first[3] != 0 || first[4] != 0 || second[1] != 0 || second[2] != 0 ||
		second[8] < 0 || second[8] > 99999999 || transform != 0 && (kind != 100 || transform < 1 || transform%2 != 1) {
		return false
	}
	switch kind {
	case 100, 110:
		return form == 0 && status == 10000
	case 116:
		return form == 0 && (status == 10000 || status == 10400)
	case 123:
		return form == 0 && (status == 10000 || status == 10200)
	case 124:
		return form == 0 && (status == 10000 || status == 0)
	case 186:
		return form == 0 && (status == 0 || status == 20000)
	case 402:
		return form == 1 && status == 0
	case 504:
		return form == 1 && status == 10001
	case 190, 192, 198, 502, 508, 510, 514:
		return form == 1 && status == 10000
	}
	return false
}

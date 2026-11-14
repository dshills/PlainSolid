package trimmed

import "plainsolid/geom"

func dualDiskRimSchedule(authored []int, limit int) ([]int, error) {
	if len(authored) == 0 {
		var out []int
		for n := 1; n <= limit; n *= 2 {
			out = append(out, n)
		}
		return out, nil
	}
	if len(authored) > 9 {
		return nil, geom.ErrInvalid
	}
	sum, previous := 0, 0
	for _, n := range authored {
		if n <= previous || n > limit || n > 256 {
			return nil, geom.ErrInvalid
		}
		sum += n
		previous = n
	}
	if sum > 512 {
		return nil, geom.ErrInvalid
	}
	return append([]int(nil), authored...), nil
}

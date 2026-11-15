package step

import (
	"math/big"

	"plainsolid/geom"
)

// MappedChainSTEPPlane records the exact coordinate change used to represent
// one original analytic STEP plane. Add UVShift to original UV coordinates
// to obtain native chart coordinates.
type MappedChainSTEPPlane struct {
	FaceID, SupportID int64
	OriginalOrigin    [3]string
	Directions        [2][3]int
	UVShift           [2]string
}

type mappedChainForeignPlane struct {
	origin [3]*big.Rat
	dir    [2][3]int
	normal [3]int
	shift  [2]*big.Rat
}

func mappedChainStepNumericTuple(f *File, id int64, kind string, dim int) ([]*big.Rat, error) {
	p, err := simple(f, id, kind, 2)
	if err != nil || p.Args[0].Kind != String || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != dim {
		return nil, geom.ErrInvalid
	}
	out := make([]*big.Rat, dim)
	for i, q := range p.Args[1].Items {
		out[i], err = affineSplineSourceRat(q)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func readMappedChainForeignFrame(f *File, id int64) (mappedChainForeignPlane, error) {
	var shell mappedChainForeignPlane
	p, err := simple(f, id, "AXIS2_PLACEMENT_3D", 4)
	if err != nil || p.Args[0].Kind != String {
		return shell, geom.ErrInvalid
	}
	refs := [3]int64{}
	for i, value := range p.Args[1:] {
		refs[i], err = ref(value)
		if err != nil {
			return shell, err
		}
	}
	origin, err := mappedChainStepPoint(f, refs[0], 3)
	if err != nil {
		return shell, err
	}
	shell.origin = [3]*big.Rat(origin)
	for i, d := range []*[3]int{&shell.normal, &shell.dir[0]} {
		q, err := mappedChainStepNumericTuple(f, refs[i+1], "DIRECTION", 3)
		if err != nil {
			return shell, err
		}
		nonzero := 0
		for j, x := range q {
			if !x.IsInt() || !x.Num().IsInt64() || x.Num().Int64() < -1 || x.Num().Int64() > 1 {
				return shell, geom.ErrConvergence
			}
			d[j] = int(x.Num().Int64())
			if d[j] != 0 {
				nonzero++
			}
		}
		if nonzero != 1 {
			return shell, geom.ErrConvergence
		}
	}
	for i := range 3 {
		a, b := (i+1)%3, (i+2)%3
		shell.dir[1][i] = shell.normal[a]*shell.dir[0][b] - shell.normal[b]*shell.dir[0][a]
	}
	if shell.dir[1] == [3]int{} {
		return shell, geom.ErrConvergence
	}
	for i, d := range shell.dir {
		shell.shift[i] = mappedChainForeignDot(shell.origin, d)
	}
	return shell, nil
}

func mappedChainForeignDot(p [3]*big.Rat, d [3]int) *big.Rat {
	result := new(big.Rat)
	for i, k := range d {
		if k != 0 {
			result.Add(result, new(big.Rat).Mul(big.NewRat(int64(k), 1), p[i]))
		}
	}
	return result
}

func (p mappedChainForeignPlane) originalUV(point [3]*big.Rat) ([2]*big.Rat, error) {
	var zero [2]*big.Rat
	if mappedChainForeignDot(point, p.normal).Cmp(mappedChainForeignDot(p.origin, p.normal)) != 0 {
		return zero, geom.ErrConvergence
	}
	var uv [2]*big.Rat
	for i, axis := range p.dir {
		uv[i] = new(big.Rat).Sub(mappedChainForeignDot(point, axis), p.shift[i])
	}
	return uv, nil
}

func (p mappedChainForeignPlane) world(native [2]*big.Rat) [3]*big.Rat {
	var model [3]*big.Rat
	for i := range model {
		model[i] = new(big.Rat)
		if p.normal[i] != 0 {
			model[i].Set(p.origin[i])
		}
		for k, axis := range p.dir {
			model[i].Add(model[i], new(big.Rat).Mul(big.NewRat(int64(axis[i]), 1), native[k]))
		}
	}
	return model
}

func readMappedChainForeignPlane(f *File, surfaceID int64, uses []analyticUse) (*geom.BSplineSurface, *mappedChainForeignPlane, error) {
	record, err := simple(f, surfaceID, "PLANE", 2)
	if err != nil || record.Args[0].Kind != String {
		return nil, nil, geom.ErrInvalid
	}
	pid, err := ref(record.Args[1])
	if err != nil {
		return nil, nil, err
	}
	p, err := readMappedChainForeignFrame(f, pid)
	if err != nil {
		return nil, nil, err
	}
	var bounds [2][2]*big.Rat
	for _, u := range uses {
		start, end, _, err := analyticEdge(f, u.id)
		if err != nil {
			return nil, nil, err
		}
		for _, vertex := range []int64{start, end} {
			point, err := mappedChainStepVertex(f, vertex)
			if err != nil {
				return nil, nil, err
			}
			uv, err := p.originalUV(point)
			if err != nil {
				return nil, nil, err
			}
			for axis, q := range uv {
				q.Add(q, p.shift[axis])
				if bounds[axis][0] == nil || q.Cmp(bounds[axis][0]) < 0 {
					bounds[axis][0] = q
				}
				if bounds[axis][1] == nil || q.Cmp(bounds[axis][1]) > 0 {
					bounds[axis][1] = q
				}
			}
		}
	}
	var knots [2][]float64
	for i := range bounds {
		n, err := mappedChainRepresentedScalars(bounds[i][:])
		if err != nil || n[0] >= n[1] {
			return nil, nil, geom.ErrConvergence
		}
		knots[i] = []float64{n[0], n[0], n[1], n[1]}
	}
	var poles []geom.Vec3
	for _, u := range bounds[0] {
		for _, v := range bounds[1] {
			point := p.world([2]*big.Rat{u, v})
			q, err := geom.RepresentedExactPoint(point)
			if err != nil {
				return nil, nil, err
			}
			poles = append(poles, q)
		}
	}
	s, err := geom.NewBSplineSurface(1, 1, 2, 2, poles, knots[0], knots[1], nil)
	return s, &p, err
}

func (p mappedChainForeignPlane) evidence(faceID, supportID int64) MappedChainSTEPPlane {
	out := MappedChainSTEPPlane{FaceID: faceID, SupportID: supportID, Directions: p.dir}
	for i, q := range p.origin {
		out.OriginalOrigin[i] = q.RatString()
	}
	for i, q := range p.shift {
		out.UVShift[i] = q.RatString()
	}
	return out
}

type mappedChainForeignLine struct {
	origin, direction []*big.Rat
}

func readMappedChainForeignLine(f *File, id int64, dim int) (mappedChainForeignLine, error) {
	var line mappedChainForeignLine
	r, err := simple(f, id, "LINE", 3)
	if err != nil || r.Args[0].Kind != String {
		return line, geom.ErrInvalid
	}
	origin, err := ref(r.Args[1])
	if err != nil {
		return line, err
	}
	line.origin, err = mappedChainStepPoint(f, origin, dim)
	if err != nil {
		return line, err
	}
	vid, err := ref(r.Args[2])
	if err != nil {
		return line, err
	}
	v, err := simple(f, vid, "VECTOR", 3)
	if err != nil || v.Args[0].Kind != String {
		return line, geom.ErrInvalid
	}
	magnitude, err := affineSplineSourceRat(v.Args[2])
	if err != nil || magnitude.Sign() <= 0 {
		return line, geom.ErrInvalid
	}
	did, err := ref(v.Args[1])
	if err != nil {
		return line, err
	}
	line.direction, err = mappedChainStepNumericTuple(f, did, "DIRECTION", dim)
	if err != nil {
		return line, err
	}
	return line, nil
}

func (l mappedChainForeignLine) parameter(p []*big.Rat) (*big.Rat, error) {
	var t *big.Rat
	for i, d := range l.direction {
		if d.Sign() != 0 {
			t = new(big.Rat).Quo(new(big.Rat).Sub(p[i], l.origin[i]), d)
			break
		}
	}
	if t == nil {
		return nil, geom.ErrDegenerate
	}
	for i, d := range l.direction {
		q := new(big.Rat).Add(l.origin[i], new(big.Rat).Mul(t, d))
		if q.Cmp(p[i]) != 0 {
			return nil, geom.ErrConvergence
		}
	}
	return t, nil
}

func (l mappedChainForeignLine) proveDirectedSpan(a, b []*big.Rat) error {
	lo, err := l.parameter(a)
	if err != nil {
		return err
	}
	hi, err := l.parameter(b)
	if err != nil {
		return err
	}
	if lo.Cmp(hi) >= 0 {
		return geom.ErrConvergence
	}
	return nil
}

package step

import (
	"fmt"
	"math/big"
	"plainsolid/geom"
)

// Rephasing a complete oriented circle changes its angular origin only. Source
// vertex endpoints plus same_sense still select the same directed minor arc.
// No rounded angular parameters or tolerance-based circle comparisons occur.
func sideDiskNormalizeRephasedCircles(f *File, canonical geom.Frame) ([]int64, error) {
	fail := func() ([]int64, error) { return nil, fmt.Errorf("step: unsupported side-disk circle rephase") }
	counts := map[int64]int{}
	for _, en := range f.Entities {
		var values []Value
		for _, r := range en.Records {
			values = append(values, r.Args...)
		}
		for len(values) > 0 {
			v := values[len(values)-1]
			values = values[:len(values)-1]
			if v.Kind == Reference {
				counts[v.Ref]++
			}
			values = append(values, v.Items...)
		}
	}
	var normalized []int64
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "EDGE_CURVE" {
			continue
		}
		edge, e := simple(f, id, "EDGE_CURVE", 5)
		if e != nil {
			return fail()
		}
		model, e := ref(edge.Args[3])
		if e != nil {
			return fail()
		}
		me := f.Entities[model]
		if len(me.Records) != 1 || me.Records[0].Type != "CIRCLE" {
			continue
		}
		circle, e := simple(f, model, "CIRCLE", 3)
		if e != nil {
			return fail()
		}
		pid, e := ref(circle.Args[1])
		if e != nil {
			return fail()
		}
		frame, e := sideDiskReadFrame(f, pid)
		if e != nil || frame.ZAxis() != canonical.ZAxis() {
			return fail()
		}
		if frame.XAxis() == canonical.XAxis() {
			continue
		}
		if len(normalized) >= 4 {
			return fail()
		}
		radius, e := cylinderSupportNumber(circle.Args[2])
		if e != nil || radius <= 0 {
			return fail()
		}
		// Compute endpoint vectors in the canonical signed frame using exact
		// rationals. Their coordinates need not be rounded back into binary64.
		var radial [2][2]*big.Rat
		for i := range radial {
			vid, e := ref(edge.Args[1+i])
			if e != nil {
				return fail()
			}
			vertex, e := simple(f, vid, "VERTEX_POINT", 2)
			if e != nil {
				return fail()
			}
			point, e := ref(vertex.Args[1])
			if e != nil {
				return fail()
			}
			p, e := splitTriple(f, point, "CARTESIAN_POINT")
			if e != nil {
				return fail()
			}
			var local [3]*big.Rat
			for j, axis := range []geom.Vec3{canonical.XAxis(), canonical.YAxis(), canonical.ZAxis()} {
				q := new(big.Rat)
				for k, x := range []float64{p.X, p.Y, p.Z} {
					delta := new(big.Rat).Sub(sideRat(x), sideRat([]float64{frame.Origin().X, frame.Origin().Y, frame.Origin().Z}[k]))
					q.Add(q, new(big.Rat).Mul(delta, sideRat([]float64{axis.X, axis.Y, axis.Z}[k])))
				}
				local[j] = q
			}
			if local[2].Sign() != 0 {
				return fail()
			}
			norm := new(big.Rat).Add(new(big.Rat).Mul(local[0], local[0]), new(big.Rat).Mul(local[1], local[1]))
			if norm.Cmp(new(big.Rat).Mul(sideRat(radius), sideRat(radius))) != 0 {
				return fail()
			}
			radial[i] = [2]*big.Rat{local[0], local[1]}
		}
		cross := new(big.Rat).Sub(new(big.Rat).Mul(radial[0][0], radial[1][1]), new(big.Rat).Mul(radial[0][1], radial[1][0]))
		same, e := boolean(edge.Args[4])
		if e != nil || cross.Sign() == 0 || same != (cross.Sign() > 0) {
			return fail()
		}
		// A nonzero cross product excludes coincident and antipodal endpoints.
		// Selecting its sign chooses the unique directed arc shorter than pi.
		placement, e := simple(f, pid, "AXIS2_PLACEMENT_3D", 4)
		if e != nil {
			return fail()
		}
		reference, e := ref(placement.Args[3])
		if e != nil {
			return fail()
		}
		if counts[model] != 1 || counts[pid] != 1 || counts[reference] != 1 {
			return fail()
		}
		direction, e := simple(f, reference, "DIRECTION", 2)
		if e != nil {
			return fail()
		}
		a := canonical.XAxis()
		direction.Args[1].Items = []Value{{Kind: Number, Text: real(a.X)}, {Kind: Number, Text: real(a.Y)}, {Kind: Number, Text: real(a.Z)}}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

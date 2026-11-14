package step

import (
	"bytes"
	"io"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// PythagoreanLocalFrame uses explicit integer direction ratios with rational
// lengths, not rounded unit vectors. X and Z must be perpendicular. Their exact
// normalization defines a proper rigid frame; Y is Z cross X. Each component is
// bounded to one million. Model points after placement must remain binary64-exact.
type PythagoreanLocalFrame struct{ Origin, XAxis, ZAxis geom.Vec3 }

// WritePythagoreanMappedPlanarMaterials rigidly places each native cardinal local
// product in its own exact rational frame before authoring mapped uses. Source
// model vertices, planes, model lines and owner pcurves remain independently
// proved. Existing cardinal mapped APIs are not widened.
func WritePythagoreanMappedPlanarMaterials(w io.Writer, m NativeMappedPlanarModel, frames []PythagoreanLocalFrame, l Limits) error {
	if w == nil || len(frames) != len(m.Products) {
		return mappedPlanarFailure()
	}
	bases := make([]mappedRationalBasis, len(frames))
	for i, frame := range frames {
		if !frame.Origin.IsFinite() {
			return mappedPlanarFailure()
		}
		for _, v := range []geom.Vec3{frame.XAxis, frame.ZAxis} {
			for _, x := range []float64{v.X, v.Y, v.Z} {
				if math.Abs(x) > 1e6 || math.Trunc(x) != x || math.IsNaN(x) {
					return mappedPlanarFailure()
				}
			}
		}
		b := mappedRationalBasis{Origin: mappedRatVec(frame.Origin), X: mappedRatVec(frame.XAxis), Z: mappedRatVec(frame.ZAxis)}
		if mappedRationalDot(b.X, b.Z).Sign() != 0 {
			return mappedPlanarFailure()
		}
		var e error
		b.X, e = mappedRationalUnit(b.X)
		if e != nil {
			return e
		}
		b.Z, e = mappedRationalUnit(b.Z)
		if e != nil {
			return e
		}
		b.Y = mappedRationalCross(b.Z, b.X)
		bases[i] = b
	}
	var raw bytes.Buffer
	if e := WriteNativeMappedPlanarMaterials(&raw, m, l); e != nil {
		return e
	}
	a, e := ReadMappedPlanarMaterials(&raw, l)
	if e != nil {
		return e
	}
	f := a.Source
	expected := make([]brep.Solid, len(m.Products))
	for i, mid := range a.MapOrder {
		b := bases[i]
		apply := func(p [3]*big.Rat, point bool) (q [3]*big.Rat) {
			for k := range q {
				q[k] = new(big.Rat)
				if point {
					q[k].Set(b.Origin[k])
				}
				for j, axis := range [][3]*big.Rat{b.X, b.Y, b.Z} {
					q[k].Add(q[k], new(big.Rat).Mul(axis[k], p[j]))
				}
			}
			return
		}
		floatPoint := func(p [3]*big.Rat) (geom.Vec3, error) {
			var v [3]float64
			for k := range v {
				var exact bool
				v[k], exact = p[k].Float64()
				if !exact || math.IsInf(v[k], 0) {
					return geom.Vec3{}, mappedPlanarFailure()
				}
			}
			return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, nil
		}
		verts := m.Products[i].Solid.Vertices()
		for j, p := range verts {
			verts[j], e = floatPoint(apply(mappedRatVec(p), true))
			if e != nil {
				return e
			}
		}
		expected[i], e = brep.NewSolid(verts, m.Products[i].Solid.Edges(), m.Products[i].Solid.Faces())
		if e != nil {
			return e
		}
		todo := []int64{a.Materials[mid].SourceBodyID}
		seen := map[int64]bool{}
		for len(todo) > 0 {
			id := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if seen[id] {
				continue
			}
			seen[id] = true
			en := f.Entities[id]
			var refs func(Value)
			refs = func(v Value) {
				if v.Kind == Reference {
					todo = append(todo, v.Ref)
				}
				for _, x := range v.Items {
					refs(x)
				}
			}
			for _, r := range en.Records {
				for _, v := range r.Args {
					refs(v)
				}
			}
			if len(en.Records) != 1 {
				continue
			}
			r := &en.Records[0]
			if (r.Type != "CARTESIAN_POINT" && r.Type != "DIRECTION") || len(r.Args) != 2 || len(r.Args[1].Items) != 3 {
				continue
			}
			p, e := mappedRationalCoordinate(f, id, r.Type, 3)
			if e != nil {
				return e
			}
			q := apply(p, r.Type == "CARTESIAN_POINT")
			if r.Type == "CARTESIAN_POINT" {
				if _, e = floatPoint(q); e != nil {
					return e
				}
			} else {
				den := big.NewInt(1)
				for _, v := range q {
					g := new(big.Int).GCD(nil, nil, den, v.Denom())
					den.Mul(new(big.Int).Quo(den, g), v.Denom())
				}
				for k := range q {
					q[k].Mul(q[k], new(big.Rat).SetInt(den))
				}
			}
			for k, v := range q {
				var s string
				if r.Type == "DIRECTION" {
					s = v.Num().String()
				} else {
					s = v.FloatString(v.Denom().BitLen() - 1)
				}
				r.Args[1].Items[k] = Value{Kind: Number, Text: s}
			}
			f.Entities[id] = en
		}
	}
	data, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	proof, e := ReadPythagoreanMappedPlanarMaterials(bytes.NewReader(data), l)
	if e != nil {
		return e
	}
	for i, mid := range proof.MapOrder {
		if !nativeMappedSameBoundary(proof.Materials[mid].Body.Solid, expected[i]) {
			return mappedPlanarFailure()
		}
	}
	n, e := w.Write(data)
	if e == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return e
}

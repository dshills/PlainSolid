package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// mixedRectangleGridSeed replaces only an independently proved identity-UV
// rectangle polygon. All boundary IDs/points remain unchanged. Opposite side
// cuts must already match exactly: no new boundary sample or tolerance weld is
// allowed. Exact positive cell areas and directed boundary incidence prove full
// coverage. Interior display rounding is recomputed in the UV fill certificate.
func mixedRectangleGridSeed(ctx context.Context, in CertifiedAffineQuadraticTrimMesh, maxTriangles int) (CertifiedAffineQuadraticTrimMesh, bool, error) {
	empty := CertifiedAffineQuadraticTrimMesh{}
	if len(in.BoundaryLoops) != 1 || len(in.BoundaryLoops[0]) < 4 || len(in.BoundaryLoops[0]) != len(in.ExactUV) {
		return in, false, nil
	}
	loop := in.BoundaryLoops[0]
	lo, hi := [2]*big.Rat{}, [2]*big.Rat{}
	for k := 0; k < 2; k++ {
		lo[k] = new(big.Rat).Set(in.ExactUV[loop[0]][k])
		hi[k] = new(big.Rat).Set(lo[k])
	}
	for _, id := range loop {
		uv := in.ExactUV[id]
		if in.ExactPoints[id][0].Cmp(uv[0]) != 0 || in.ExactPoints[id][1].Cmp(uv[1]) != 0 || in.ExactPoints[id][2].Sign() != 0 {
			return empty, false, geom.ErrInvalid
		}
		for k := 0; k < 2; k++ {
			if uv[k].Cmp(lo[k]) < 0 {
				lo[k].Set(uv[k])
			}
			if uv[k].Cmp(hi[k]) > 0 {
				hi[k].Set(uv[k])
			}
		}
	}
	if lo[0].Cmp(hi[0]) >= 0 || lo[1].Cmp(hi[1]) >= 0 {
		return in, false, nil
	}
	sides := [4]map[string]*big.Rat{}
	for i := range sides {
		sides[i] = map[string]*big.Rat{}
	}
	key := func(u, v *big.Rat) string { return u.RatString() + "," + v.RatString() }
	indices := map[string]int{}
	for j, id := range loop {
		a, b := in.ExactUV[id], in.ExactUV[loop[(j+1)%len(loop)]]
		valid := false
		for axis := 0; axis < 2; axis++ {
			for end, value := range []*big.Rat{lo[axis], hi[axis]} {
				if a[axis].Cmp(value) == 0 {
					sides[axis*2+end][a[1-axis].RatString()] = a[1-axis]
					if b[axis].Cmp(value) == 0 {
						valid = true
					}
				}
			}
		}
		if !valid {
			return in, false, nil
		}
		indices[key(a[0], a[1])] = id
	}
	for axis := 0; axis < 2; axis++ {
		a, b := sides[axis*2], sides[axis*2+1]
		if len(a) != len(b) {
			return in, false, nil
		}
		for k := range a {
			if _, ok := b[k]; !ok {
				return in, false, nil
			}
		}
	}
	sorted := func(s map[string]*big.Rat) []*big.Rat {
		var r []*big.Rat
		for _, x := range s {
			r = append(r, new(big.Rat).Set(x))
		}
		sort.Slice(r, func(i, j int) bool { return r[i].Cmp(r[j]) < 0 })
		return r
	}
	us, vs := sorted(sides[2]), sorted(sides[0])
	if len(us) < 2 || len(vs) < 2 {
		return in, false, nil
	}
	if (len(us) - 1) > maxTriangles/2/(len(vs)-1) {
		return empty, false, geom.ErrConvergence
	}
	r := in
	r.Mesh.Vertices = append([]geom.Vec3(nil), in.Mesh.Vertices...)
	r.Mesh.Triangles = nil
	r.ExactUV = append([][2]*big.Rat(nil), in.ExactUV...)
	r.ExactPoints = append([][3]*big.Rat(nil), in.ExactPoints...)
	r.OriginalFace = nil
	grid := make([][]int, len(us))
	for i, u := range us {
		grid[i] = make([]int, len(vs))
		for j, v := range vs {
			if e := ctx.Err(); e != nil {
				return empty, false, e
			}
			id, ok := indices[key(u, v)]
			if !ok {
				if i == 0 || j == 0 || i == len(us)-1 || j == len(vs)-1 {
					return empty, false, geom.ErrConvergence
				}
				id = len(r.Mesh.Vertices)
				uf, _ := u.Float64()
				vf, _ := v.Float64()
				r.Mesh.Vertices = append(r.Mesh.Vertices, geom.Vec3{X: uf, Y: vf})
				r.ExactUV = append(r.ExactUV, [2]*big.Rat{new(big.Rat).Set(u), new(big.Rat).Set(v)})
				r.ExactPoints = append(r.ExactPoints, [3]*big.Rat{new(big.Rat).Set(u), new(big.Rat).Set(v), new(big.Rat)})
			}
			grid[i][j] = id
		}
	}
	for i := 0; i+1 < len(us); i++ {
		for j := 0; j+1 < len(vs); j++ {
			a, b, c, d := grid[i][j], grid[i+1][j], grid[i+1][j+1], grid[i][j+1]
			r.Mesh.Triangles = append(r.Mesh.Triangles, [3]int{a, b, c}, [3]int{a, c, d})
			r.OriginalFace = append(r.OriginalFace, -1, -1)
		}
	}
	directed := map[[2]int]int{}
	area := new(big.Rat)
	for _, t := range r.Mesh.Triangles {
		a, b, c := r.ExactUV[t[0]], r.ExactUV[t[1]], r.ExactUV[t[2]]
		cross := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(c[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(c[0], a[0])))
		if cross.Sign() <= 0 {
			return empty, false, geom.ErrConvergence
		}
		area.Add(area, cross)
		for j, id := range t {
			directed[[2]int{id, t[(j+1)%3]}]++
		}
	}
	area.Quo(area, big.NewRat(2, 1))
	if area.Cmp(in.Region.PolygonUVArea) != 0 {
		return empty, false, geom.ErrConvergence
	}
	expected := map[[2]int]bool{}
	for j, id := range loop {
		e := [2]int{id, loop[(j+1)%len(loop)]}
		expected[e] = true
		if directed[e] != 1 || directed[[2]int{e[1], e[0]}] != 0 {
			return empty, false, geom.ErrConvergence
		}
	}
	for e, n := range directed {
		if n != 1 || (!expected[e] && directed[[2]int{e[1], e[0]}] != 1) {
			return empty, false, geom.ErrConvergence
		}
	}
	r.DisplaySquaredDistance = new(big.Rat)
	for id, p := range r.Mesh.Vertices {
		sq := new(big.Rat)
		for k, f := range []float64{p.X, p.Y, p.Z} {
			d := new(big.Rat).Sub(r.ExactPoints[id][k], ratInset(f))
			sq.Add(sq, new(big.Rat).Mul(d, d))
		}
		if sq.Cmp(r.DisplaySquaredDistance) > 0 {
			r.DisplaySquaredDistance.Set(sq)
		}
	}
	r.FillSquaredDistance = new(big.Rat).Set(in.Region.FillSquaredDistance)
	if r.DisplaySquaredDistance.Sign() > 0 {
		r.FillSquaredDistance.Add(r.FillSquaredDistance, r.DisplaySquaredDistance)
		r.FillSquaredDistance.Mul(r.FillSquaredDistance, big.NewRat(2, 1))
	}
	if e := r.Mesh.Validate(); e != nil {
		return empty, false, e
	}
	return r, true, nil
}

package trimmed

import (
	"context"
	"plainsolid/geom"
	"plainsolid/intersect"
	"runtime"
)

// nativeExactTubeValidationFace substitutes only independently proved affine-UV
// tube paths in a temporary UV-topology view. Original native storage remains
// unchanged. This is not sampled polyline validation of implicit boundaries.
func nativeExactTubeValidationFace(ctx context.Context, s Solid, f Face) (Face, error) {
	out := f
	out.Outer = append([]Use(nil), f.Outer...)
	out.Holes = make([][]Use, len(f.Holes))
	for i, h := range f.Holes {
		out.Holes[i] = append([]Use(nil), h...)
	}
	remaining := 8000000
	count := 0
	loops := []*[]Use{&out.Outer}
	for i := range out.Holes {
		loops = append(loops, &out.Holes[i])
	}
	for _, loop := range loops {
		for i, u := range *loop {
			curve := u.Curve
			reversed := false
			if r, ok := curve.(reverseTrim); ok {
				curve = r.differentiableTrim
				reversed = true
			}
			tube, ok := curve.(intersect.TubeTrim)
			if !ok {
				continue
			}
			count++
			if count > 256 || remaining < 1 {
				return Face{}, geom.ErrConvergence
			}
			if u.Edge < 0 || u.Edge >= len(s.edges) || !sameCertifiedSection(s.edges[u.Edge], f.Surface, u) || !tube.HasSourceSurface(f.Surface) {
				return Face{}, geom.ErrConvergence
			}
			proof, e := tube.CertifyAffinePath(ctx, geom.TubeAffineLineOptions{MaxOperations: remaining, MaxPoles: 4096, MaxCoefficientBits: 16384})
			remaining -= proof.ExactOperations
			if e != nil {
				return Face{}, e
			}
			if !proof.Proven {
				return Face{}, geom.ErrConvergence
			}
			_, _, second := tube.Recipe()
			offset := 0
			if second {
				offset = 2
			}
			start, end := proof.Start, proof.End
			modelStart, modelEnd := proof.ModelStart, proof.ModelEnd
			if reversed {
				start, end = end, start
				modelStart, modelEnd = modelEnd, modelStart
			}
			for j, uv := range [2]geom.Vec2{u.UVStart, u.UVEnd} {
				p := start
				if j == 1 {
					p = end
				}
				if p[offset].Cmp(ratInset(uv.X)) != 0 || p[offset+1].Cmp(ratInset(uv.Y)) != 0 {
					return Face{}, geom.ErrUnrepresentable
				}
				runtime.KeepAlive(&p) // golang/go#82026: keeps p in the stack map. Remove once the Go fix is in use.
			}
			edge := s.edges[u.Edge]
			ids := [2]int{edge.Start, edge.End}
			if u.Reversed {
				ids[0], ids[1] = ids[1], ids[0]
			}
			for j, id := range ids {
				if id < 0 || id >= len(s.vertices) {
					return Face{}, geom.ErrInvalid
				}
				p := modelStart
				if j == 1 {
					p = modelEnd
				}
				exact, e := geom.RepresentedExactPoint(p)
				if e != nil {
					return Face{}, e
				}
				if exact != s.vertices[id] {
					return Face{}, geom.ErrConvergence
				}
			}
			(*loop)[i].Curve = nil
		}
	}
	return out, nil
}

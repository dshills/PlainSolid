package trimmed

import "plainsolid/geom"

func tubeLensPartitionSurface(f AlgebraicTubeFacePartition) geom.Surface {
	if f.SourceSurface != nil {
		return f.SourceSurface
	}
	return f.Support
}

// Exact adapters never pass through Data().Poles: only their homogeneous net
// is permuted. The legacy B-spline path intentionally keeps its old evaluator
// and normalization behavior; returned Support continues to denote that source.
func permuteTubeLensSupport(f AlgebraicTubeFacePartition, axes [3]int, o geom.SurfacePatchOptions) (geom.Surface, *geom.SurfacePatch, error) {
	source := tubeLensPartitionSurface(f)
	if q, ok := source.(*geom.ExactBezierSurface); ok {
		patch, e := q.ExactPatch().WithLimits(o)
		if e != nil {
			return nil, nil, e
		}
		var m geom.Affine
		for i, j := range axes {
			m[i][j] = 1
		}
		patch, e = patch.Transformed(m)
		if e != nil {
			return nil, nil, e
		}
		out, e := geom.NewExactBezierSurface(patch)
		return out, patch, e
	}
	s, ok := source.(*geom.BSplineSurface)
	if !ok || s == nil {
		return nil, nil, geom.ErrConvergence
	}
	d := s.Data()
	for i, p := range d.Poles {
		v := [3]float64{p.X, p.Y, p.Z}
		d.Poles[i] = geom.Vec3{X: v[axes[0]], Y: v[axes[1]], Z: v[axes[2]]}
	}
	out, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		return nil, nil, e
	}
	patch, e := geom.NewSurfacePatch(out, o)
	return out, patch, e
}

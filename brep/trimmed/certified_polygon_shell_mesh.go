package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedPolygonShellOptions keeps the face approximation/evaluation and
// edge proof budgets per face/use. MaxTriangles bounds the final shell (default
// 1M, maximum 8M). At most two mesh passes are made per face; their triangle
// checks share Face.MaxTriangleChecks. No existing tessellation defaults change.
type CertifiedPolygonShellOptions struct {
	Face         CertifiedPatchMeshOptions
	Edges        edgecheck.Options
	MaxTriangles int
}

// CertifiedPolygonShellMesh is a conforming closed mesh of straight-trim spline
// faces. OriginalFace and SquaredErrors correspond to triangles. Shared samples
// are welded by original topological identities, not geometric proximity.
// BothDirections has the same per-face interpolation meaning as the patch API.
// This does not certify embedding of the smooth shell or of its approximation.
type CertifiedPolygonShellMesh struct {
	Mesh           mesh.Mesh
	OriginalFace   []int
	SquaredErrors  []*big.Rat
	UVTriangles    [][3]geom.Vec2
	BothDirections bool
	Subdivisions   int
	TriangleChecks int
}

// TessellatePolygonShellCertified requires exact continuous same-parameter
// agreement of every edge use. All faces use a common dyadic subdivision count,
// and every shared vertex/sample must round identically from exact evaluation.
// Closed/collapsed edges and unresolved parameterizations reject. This selected
// subset supports polygon holes and low-degree rational curved supports without
// replacing a curved trim by an uncertified straight segment. Failure is atomic.
func (s Solid) TessellatePolygonShellCertified(ctx context.Context, o CertifiedPolygonShellOptions) (CertifiedPolygonShellMesh, error) {
	empty := CertifiedPolygonShellMesh{}
	if ctx == nil || len(s.faces) == 0 {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 8000000 {
		return empty, geom.ErrInvalid
	}
	if o.Face.MaxTriangleChecks == 0 {
		o.Face.MaxTriangleChecks = 100000
	}
	// Reject an impossible output budget before any geometric proof or mesh.
	minimumTriangles := 0
	for _, f := range s.faces {
		count := len(f.Outer) + 2*len(f.Holes) - 2
		for _, h := range f.Holes {
			count += len(h)
		}
		if count < 1 || count > o.MaxTriangles-minimumTriangles {
			return empty, geom.ErrConvergence
		}
		minimumTriangles += count
	}
	uses := make([]int, len(s.edges))
	for _, f := range s.faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				if u.Edge < 0 || u.Edge >= len(s.edges) || u.Curve != nil {
					return empty, geom.ErrConvergence
				}
				e := s.edges[u.Edge]
				if e.Start == e.End || e.Start < 0 || e.End < 0 || e.Start >= len(s.vertices) || e.End >= len(s.vertices) {
					return empty, geom.ErrConvergence
				}
				uses[u.Edge]++
			}
		}
	}
	for _, n := range uses {
		if n != 2 {
			return empty, geom.ErrConvergence
		}
	}
	proof, err := s.ValidateEdgeAgreement(ctx, o.Edges)
	if err != nil {
		return empty, err
	}
	if !proof.Complete {
		return empty, geom.ErrConvergence
	}
	for _, u := range proof.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return empty, geom.ErrConvergence
		}
	}
	faces := make([]CertifiedPolygonPatchMesh, len(s.faces))
	n := 1
	checks := 0
	for i := range faces {
		faces[i], err = s.TessellatePolygonFaceCertified(ctx, i, o.Face)
		if err != nil {
			return empty, err
		}
		checks += faces[i].TriangleChecks
		if faces[i].Subdivisions > n {
			n = faces[i].Subdivisions
		}
	}
	// A larger common level is not assumed to succeed: each face is re-certified.
	for i := range faces {
		if faces[i].Subdivisions != n {
			opt := o.Face
			opt.MaxTriangleChecks -= faces[i].TriangleChecks
			opt.MaxSubdivisions = n
			if opt.MaxTriangleChecks <= 0 {
				return empty, geom.ErrConvergence
			}
			faces[i], err = s.tessellatePolygonFaceCertified(ctx, i, opt, n)
			if err != nil {
				return empty, err
			}
			checks += faces[i].TriangleChecks
		}
	}
	result := CertifiedPolygonShellMesh{BothDirections: true, Subdivisions: n, TriangleChecks: checks}
	// kind 0: original vertex; kind 1: edge sample; kind 2: face interior.
	type identity struct{ kind, a, b int }
	ids := map[identity]int{}
	for fi, fm := range faces {
		if len(fm.Mesh.Triangles) > o.MaxTriangles-len(result.Mesh.Triangles) {
			return empty, geom.ErrConvergence
		}
		keys := make([]identity, len(fm.Mesh.Vertices))
		for i := range keys {
			keys[i] = identity{2, fi, i}
		}
		f := s.faces[fi]
		loops := append([][]Use{f.Outer}, f.Holes...)
		for li, loop := range loops {
			if len(fm.Loops[li]) != len(loop)*n {
				return empty, geom.ErrConvergence
			}
			for j, u := range loop {
				for k := 0; k < n; k++ {
					sample := k
					if u.Reversed {
						sample = n - k
					}
					e := s.edges[u.Edge]
					key := identity{1, u.Edge, sample}
					if sample == 0 {
						key = identity{0, e.Start, 0}
					}
					if sample == n {
						key = identity{0, e.End, 0}
					}
					id := fm.Loops[li][j*n+k]
					if keys[id].kind != 2 && keys[id] != key {
						return empty, geom.ErrConvergence
					}
					keys[id] = key
				}
			}
		}
		remap := make([]int, len(keys))
		for i, key := range keys {
			if id, ok := ids[key]; ok {
				if result.Mesh.Vertices[id] != fm.Mesh.Vertices[i] {
					return empty, geom.ErrConvergence
				}
				remap[i] = id
			} else {
				id := len(result.Mesh.Vertices)
				ids[key] = id
				remap[i] = id
				result.Mesh.Vertices = append(result.Mesh.Vertices, fm.Mesh.Vertices[i])
			}
		}
		for i, t := range fm.Mesh.Triangles {
			result.Mesh.Triangles = append(result.Mesh.Triangles, [3]int{remap[t[0]], remap[t[1]], remap[t[2]]})
			result.OriginalFace = append(result.OriginalFace, fi)
			result.UVTriangles = append(result.UVTriangles, fm.UVTriangles[i])
			result.SquaredErrors = append(result.SquaredErrors, fm.SquaredErrors[i])
		}
		result.BothDirections = result.BothDirections && fm.BothDirections
	}
	if err := result.Mesh.ValidateSolid(); err != nil {
		return empty, err
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	return result, nil
}

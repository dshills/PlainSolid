package parametric

import (
	"context"
	"errors"
	"fmt"
	"math"

	"plainsolid/geom"
	"plainsolid/mesh"
)

// TessellationOptions describes sampled model-space chord deviation and
// optional angular deviation (radians). Subdivisions refine dyadically from 4
// up to MaxSubdivisions, default 256. MaxTriangles defaults to one million and
// caps the grid's pre-collapse triangle count.
type TessellationOptions struct {
	Deflection, Angle             float64
	MaxSubdivisions, MaxTriangles int
	// AnalyticBound requires the real-arithmetic DeflectionBound in addition
	// to sample checks for supported analytic and clamped spline patches.
	AnalyticBound bool
}

var errRefine = errors.New("parametric: samples require a finer grid")

// Tessellate creates a conforming indexed boundary. Edge samples are shared by
// topology, including seams; all samples on a pole map to its vertex. Chord
// error is checked at edge midpoints and triangle centroids, not certified over
// arbitrary input surfaces. Angle, when positive, compares each emitted
// triangle's normal against the oriented surface normal at its UV centroid.
func (s Solid) Tessellate(ctx context.Context, o TessellationOptions) (mesh.Mesh, error) {
	if ctx == nil || o.Deflection <= 0 || !finite(o.Deflection) || !finite(o.Angle) || o.Angle < 0 || o.Angle > math.Pi || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 1024 || o.MaxTriangles < 0 {
		return mesh.Mesh{}, fmt.Errorf("parametric: invalid tessellation options")
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 256
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	if err := ctx.Err(); err != nil {
		return mesh.Mesh{}, err
	}
	if len(s.faces) == 0 {
		return mesh.Mesh{}, nil
	}
	bound := 0.
	if o.AnalyticBound {
		var e error
		bound, e = s.deflectionCurvature(ctx)
		if e != nil {
			return mesh.Mesh{}, e
		}
	}
	for n := 4; n <= o.MaxSubdivisions; n *= 2 {
		if len(s.faces) > o.MaxTriangles/(2*n*n) {
			break
		}
		if o.AnalyticBound {
			if upAdd(upDiv(upDiv(bound, float64(n)), float64(n)), s.tolerance) > o.Deflection {
				continue
			}
		}
		m, err := s.gridContext(ctx, n, true, o.Deflection, o.Angle)
		if !errors.Is(err, errRefine) {
			return m, err
		}
	}
	return mesh.Mesh{}, ErrBudget
}

func (s Solid) grid(n int, check bool, deflection, angle float64) (mesh.Mesh, error) {
	return s.gridContext(context.Background(), n, check, deflection, angle)
}
func (s Solid) gridContext(ctx context.Context, n int, check bool, deflection, angle float64) (mesh.Mesh, error) {
	return s.gridWithDivisions(ctx, n, nil, check, deflection, angle)
}

func (s Solid) gridWithDivisions(ctx context.Context, nu int, vertical []int, check bool, deflection, angle float64) (mesh.Mesh, error) {
	m := mesh.Mesh{Vertices: s.Vertices()}
	divisions := make([]int, len(s.edges))
	for i, f := range s.faces {
		nv := nu
		if vertical != nil {
			nv = vertical[i]
		}
		for side, use := range f.Boundary {
			n := nu
			if side%2 == 1 {
				n = nv
			}
			if divisions[use.Edge] != 0 && divisions[use.Edge] != n {
				return mesh.Mesh{}, ErrTopology
			}
			divisions[use.Edge] = n
		}
	}
	boundaries := make([][]int, len(s.edges))
	for i, e := range s.edges {
		if err := ctx.Err(); err != nil {
			return mesh.Mesh{}, err
		}
		n := divisions[i]
		row := make([]int, n+1)
		row[0], row[n] = e.Start, e.End
		for j := 1; j < n; j++ {
			if e.Curve == nil {
				row[j] = e.Start
				continue
			}
			p, err := s.edgePoint(i, float64(j)/float64(n))
			if err != nil || !p.IsFinite() {
				return mesh.Mesh{}, fmt.Errorf("%w: edge %d sample", ErrTopology, i)
			}
			row[j] = len(m.Vertices)
			m.Vertices = append(m.Vertices, p)
		}
		boundaries[i] = row
	}
	for fi, f := range s.faces {
		nv := nu
		if vertical != nil {
			nv = vertical[fi]
		}
		stride := nu + 1
		grid := make([]int, stride*(nv+1))
		for j := 0; j <= nv; j++ {
			if err := ctx.Err(); err != nil {
				return mesh.Mesh{}, err
			}
			for i := 0; i <= nu; i++ {
				side, k := -1, 0
				switch {
				case j == 0:
					side, k = 0, i
				case i == nu:
					side, k = 1, j
				case j == nv:
					side, k = 2, nu-i
				case i == 0:
					side, k = 3, nv-j
				}
				u, v := float64(i)/float64(nu), float64(j)/float64(nv)
				p, err := f.point(u, v)
				if err != nil || !p.IsFinite() {
					return mesh.Mesh{}, fmt.Errorf("%w: face %d sample", ErrTopology, fi)
				}
				if side >= 0 {
					use := f.Boundary[side]
					if use.Reversed {
						k = len(boundaries[use.Edge]) - 1 - k
					}
					id := boundaries[use.Edge][k]
					if !near(p, m.Vertices[id], s.tolerance) {
						return mesh.Mesh{}, fmt.Errorf("%w: face %d boundary sample mismatch", ErrTopology, fi)
					}
					grid[j*stride+i] = id
				} else {
					grid[j*stride+i] = len(m.Vertices)
					m.Vertices = append(m.Vertices, p)
				}
			}
		}
		for j := 0; j < nv; j++ {
			if err := ctx.Err(); err != nil {
				return mesh.Mesh{}, err
			}
			for i := 0; i < nu; i++ {
				a := j*stride + i
				for _, tri := range [][3]int{{a, a + 1, a + stride + 1}, {a, a + stride + 1, a + stride}} {
					ids := [3]int{grid[tri[0]], grid[tri[1]], grid[tri[2]]}
					collapsed := ids[0] == ids[1] || ids[1] == ids[2] || ids[0] == ids[2]
					if check {
						var uv [3][2]float64
						for k, p := range tri {
							uv[k] = [2]float64{float64(p%stride) / float64(nu), float64(p/stride) / float64(nv)}
						}
						p := [3]geom.Vec3{m.Vertices[ids[0]], m.Vertices[ids[1]], m.Vertices[ids[2]]}
						if err := f.checkTriangle(uv, p, deflection, angle, collapsed); err != nil {
							return mesh.Mesh{}, err
						}
					}
					if collapsed {
						continue
					}
					if f.Reversed {
						ids[1], ids[2] = ids[2], ids[1]
					}
					m.Triangles = append(m.Triangles, ids)
				}
			}
		}
	}
	if err := m.ValidateSolid(); err != nil {
		return mesh.Mesh{}, err
	}
	return m, nil
}

func (f Face) checkTriangle(uv [3][2]float64, p [3]geom.Vec3, deflection, angle float64, collapsed bool) error {
	for i := 0; i < 4; i++ {
		var u, v float64
		var target geom.Vec3
		if i == 3 {
			u = (uv[0][0] + uv[1][0] + uv[2][0]) / 3
			v = (uv[0][1] + uv[1][1] + uv[2][1]) / 3
			target = p[0].Scale(1.0 / 3).Add(p[1].Scale(1.0 / 3)).Add(p[2].Scale(1.0 / 3))
		} else {
			j := (i + 1) % 3
			u = (uv[i][0] + uv[j][0]) / 2
			v = (uv[i][1] + uv[j][1]) / 2
			target = p[i].Scale(.5).Add(p[j].Scale(.5))
		}
		q, err := f.point(u, v)
		if err != nil || !q.IsFinite() {
			return fmt.Errorf("%w: face interior evaluation", ErrTopology)
		}
		if !near(q, target, deflection) {
			return errRefine
		}
		if i == 3 && angle > 0 && !collapsed {
			du, dv, err := f.Surface.Partials(f.U.at(u), f.V.at(v))
			if err != nil {
				return err
			}
			a, err := unitCross(du, dv)
			if err != nil {
				return fmt.Errorf("%w: singular interior normal", ErrTopology)
			}
			b, err := unitCross(p[1].Sub(p[0]), p[2].Sub(p[0]))
			if err != nil {
				return errRefine
			}
			if math.Atan2(a.Cross(b).Length(), a.Dot(b)) > angle {
				return errRefine
			}
		}
	}
	return nil
}

func unitCross(a, b geom.Vec3) (geom.Vec3, error) {
	u, e := a.Unit()
	if e != nil {
		return geom.Vec3{}, e
	}
	v, e := b.Unit()
	if e != nil {
		return geom.Vec3{}, e
	}
	return u.Cross(v).Unit()
}

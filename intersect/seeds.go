package intersect

import (
	"context"
	"fmt"
	"math"

	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// SeedOptions bounds the uniform patch sampling and narrowphase work. Grid
// defaults to 12 cells in each direction; MaxSeeds defaults to 64.
// Tolerance is the world-space correction residual.
type SeedOptions struct {
	Grid, MaxSeeds, MaxPairs int
	Tolerance                float64
}

// SeedReport contains corrected transverse candidates. Unresolved counts mesh
// contacts that could not be corrected, including coplanar triangle overlap.
// Truncated indicates a work or output limit. The report does not certify that
// all branches were discovered: small, tangent or high-curvature intersections
// may be missed by the sampled patch meshes.
type SeedReport struct {
	Samples    []Sample
	Unresolved int
	Truncated  bool
}

// FindSeeds uses triangle overlap as a broad candidate generator, then
// corrects candidates on the original surfaces. It accepts regular rectangular
// patches with nondegenerate sampled triangles. Coincident or singular
// regions produce unresolved counts. Trace can continue from Samples.
func FindSeeds(ctx context.Context, a, b Patch, o SeedOptions) (SeedReport, error) {
	var out SeedReport
	if ctx == nil || !a.valid() || !b.valid() || o.Grid < 0 || o.Grid > 64 || o.MaxSeeds < 0 || o.MaxSeeds > 4096 || o.MaxPairs < 0 ||
		o.Tolerance <= 0 || !finite(o.Tolerance) {
		return out, fmt.Errorf("intersect: invalid seed options")
	}
	if o.Grid == 0 {
		o.Grid = 12
	}
	if o.MaxSeeds == 0 {
		o.MaxSeeds = 64
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	ma, ua, e := samplePatch(ctx, a, o.Grid)
	if e != nil {
		return out, e
	}
	mb, ub, e := samplePatch(ctx, b, o.Grid)
	if e != nil {
		return out, e
	}
	contacts, e := collision.MeshIntersections(ma, mb, collision.Options{MaxPairs: o.MaxPairs, MaxContacts: 10000})
	if e != nil && e != collision.ErrLimit {
		return out, e
	}
	out.Truncated = !contacts.Complete
	for _, c := range contacts.Contacts {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		if c.Kind == collision.CoplanarOverlap || len(c.Points) == 0 {
			out.Unresolved++
			continue
		}
		p := c.Points[0]
		if len(c.Points) > 1 {
			p = p.Scale(.5).Add(c.Points[1].Scale(.5))
		}
		u, e := interpolateUV(ma, ua, c.TriangleA, p)
		if e != nil {
			out.Unresolved++
			continue
		}
		v, e := interpolateUV(mb, ub, c.TriangleB, p)
		if e != nil {
			out.Unresolved++
			continue
		}
		params := Parameters{A: u, B: v}
		f, e := evaluate(a, b, params)
		if e != nil {
			out.Unresolved++
			continue
		}
		params, f, e = correct(ctx, a, b, params, p, f.t, Options{Tolerance: o.Tolerance, MaxNewton: 20})
		if e != nil {
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
			out.Unresolved++
			continue
		}
		x := f.p.Scale(.5).Add(f.q.Scale(.5))
		duplicate := false
		for _, y := range out.Samples {
			if x.Sub(y.Point).Length() <= o.Tolerance*2 {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		if len(out.Samples) == o.MaxSeeds {
			out.Truncated = true
			break
		}
		out.Samples = append(out.Samples, Sample{params, x, f.t, f.p.Sub(f.q).Length()})
	}
	return out, nil
}

func samplePatch(ctx context.Context, p Patch, n int) (mesh.Mesh, []geom.Vec2, error) {
	count := (n + 1) * (n + 1)
	m := mesh.Mesh{Vertices: make([]geom.Vec3, 0, count), Triangles: make([][3]int, 0, 2*n*n)}
	uv := make([]geom.Vec2, 0, count)
	mix := func(a, b float64, i int) float64 {
		if i == n {
			return b
		}
		return a + (b-a)*(float64(i)/float64(n))
	}
	for j := 0; j <= n; j++ {
		if e := ctx.Err(); e != nil {
			return mesh.Mesh{}, nil, e
		}
		for i := 0; i <= n; i++ {
			u, v := mix(p.Min.X, p.Max.X, i), mix(p.Min.Y, p.Max.Y, j)
			x, e := p.Surface.Eval(u, v)
			if e != nil {
				return mesh.Mesh{}, nil, e
			}
			m.Vertices = append(m.Vertices, x)
			uv = append(uv, geom.Vec2{X: u, Y: v})
			if i > 0 && j > 0 {
				d := j*(n+1) + i
				a := d - n - 2
				m.Triangles = append(m.Triangles, [3]int{a, a + 1, d}, [3]int{a, d, d - 1})
			}
		}
	}
	return m, uv, m.Validate()
}
func interpolateUV(m mesh.Mesh, uv []geom.Vec2, i int, p geom.Vec3) (geom.Vec2, error) {
	t := m.Triangles[i]
	o := m.Vertices[t[0]]
	a, b, c := m.Vertices[t[1]].Sub(o), m.Vertices[t[2]].Sub(o), p.Sub(o)
	scale := math.Max(a.Length(), b.Length())
	if scale <= 0 || !finite(scale) || !finite(1/scale) {
		return geom.Vec2{}, ErrSingular
	}
	a, b, c = a.Scale(1/scale), b.Scale(1/scale), c.Scale(1/scale)
	aa, bb, ab := a.Dot(a), b.Dot(b), a.Dot(b)
	d := a.Cross(b).Length()
	if d < 1e-12 {
		return geom.Vec2{}, ErrSingular
	}
	ac, bc := a.Dot(c), b.Dot(c)
	x, y := (ac*bb-bc*ab)/(d*d), (bc*aa-ac*ab)/(d*d)
	if !finite(x) || !finite(y) || x < -1e-7 || y < -1e-7 || x+y > 1+1e-7 {
		return geom.Vec2{}, ErrUnresolved
	}
	x, y = max(0, x), max(0, y)
	if x+y > 1 {
		x, y = x/(x+y), y/(x+y)
	}
	w := max(0, 1-x-y)
	return geom.Vec2{X: w*uv[t[0]].X + x*uv[t[1]].X + y*uv[t[2]].X, Y: w*uv[t[0]].Y + x*uv[t[1]].Y + y*uv[t[2]].Y}, nil
}

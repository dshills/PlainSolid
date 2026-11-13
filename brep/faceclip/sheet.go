package faceclip

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/mesh"
)

var ErrConversion = errors.New("faceclip: sheet conversion cannot preserve planar boundary")

type SheetOptions struct {
	MaxDisplacement float64
	MaxTriangles    int
	Validation      LoopOptions
}

// Sheet contains an open triangle mesh, caller-owned exact coordinates aligned
// with mesh vertices, and original loop vertex to mesh index maps. No new
// geometric vertices are introduced; collinear boundary points are retained.
type Sheet struct {
	Mesh                   mesh.Mesh
	ExactVertices          []Point
	LoopVertices           [][]int
	MaxSquaredDisplacement *big.Rat
}

// TriangulateSheet validates exact loops, checks bounded float conversion and
// validates the converted loops again. Rounded vertices must remain exactly on
// the original plane; original collinear triples must remain collinear. Holes,
// nesting, orientation and distinct vertex identity must survive. This certifies
// these incidence properties, not zero coordinate error or renderer roundoff.
func TriangulateSheet(ctx context.Context, b Boundary, o SheetOptions) (Sheet, error) {
	empty := Sheet{}
	if o.MaxDisplacement < 0 || math.IsNaN(o.MaxDisplacement) || math.IsInf(o.MaxDisplacement, 0) || o.MaxTriangles < 0 {
		return empty, ErrInvalid
	}
	if o.MaxDisplacement == 0 {
		o.MaxDisplacement = 1e-9
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 16384
	}
	if e := ValidateBoundary(ctx, b, o.Validation); e != nil {
		return empty, e
	}
	out := Sheet{MaxSquaredDisplacement: new(big.Rat)}
	if len(b.Loops) == 0 {
		return out, nil
	}
	limit := o.Validation.MaxWork
	if limit == 0 {
		limit = 2000000
	}
	work := 0
	tick := func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		work++
		if work > limit {
			return ErrLimit
		}
		return nil
	}
	rounded := Boundary{Axis: b.Axis, ProjectedArea: new(big.Rat)}
	seen := map[geom.Vec3]int{}
	tol2 := mul(n(o.MaxDisplacement), n(o.MaxDisplacement))
	origin := b.Loops[0].Vertices[0]
	var normal Point
	for i := 1; i+1 < len(b.Loops[0].Vertices); i++ {
		normal = cross(diff(b.Loops[0].Vertices[i], origin), diff(b.Loops[0].Vertices[i+1], origin))
		if normal[b.Axis].Sign() != 0 {
			break
		}
	}
	planeD := dot(normal, origin)
	u, v := (b.Axis+1)%3, (b.Axis+2)%3
	for _, l := range b.Loops {
		rl := Loop{Hole: l.Hole, Parent: l.Parent}
		var ids []int
		for _, p := range l.Vertices {
			if e := tick(); e != nil {
				return empty, e
			}
			var xyz [3]float64
			for k := range p {
				xyz[k], _ = p[k].Float64()
			}
			x := geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
			if !x.IsFinite() {
				return empty, ErrConversion
			}
			if _, ok := seen[x]; ok {
				return empty, ErrConversion
			}
			q := vec(x)
			d := diff(q, p)
			error2 := dot(d, d)
			if error2.Cmp(tol2) > 0 || dot(normal, q).Cmp(planeD) != 0 {
				return empty, ErrConversion
			}
			if error2.Cmp(out.MaxSquaredDisplacement) > 0 {
				out.MaxSquaredDisplacement.Set(error2)
			}
			seen[x] = len(out.Mesh.Vertices)
			ids = append(ids, len(out.Mesh.Vertices))
			out.Mesh.Vertices = append(out.Mesh.Vertices, x)
			out.ExactVertices = append(out.ExactVertices, clone(p))
			rl.Vertices = append(rl.Vertices, q)
		}
		out.LoopVertices = append(out.LoopVertices, ids)
		rounded.Loops = append(rounded.Loops, rl)
	}
	area := new(big.Rat)
	for li, l := range b.Loops {
		rl := rounded.Loops[li]
		for i, p := range l.Vertices {
			j, k := (i+1)%len(l.Vertices), (i+2)%len(l.Vertices)
			old := cross(diff(l.Vertices[j], p), diff(l.Vertices[k], p))[b.Axis]
			now := cross(diff(rl.Vertices[j], rl.Vertices[i]), diff(rl.Vertices[k], rl.Vertices[i]))[b.Axis]
			if old.Sign() != now.Sign() {
				return empty, ErrConversion
			}
			a, c := rl.Vertices[i], rl.Vertices[j]
			area.Add(area, sub(mul(a[u], c[v]), mul(a[v], c[u])))
		}
	}
	rounded.ProjectedArea.Quo(new(big.Rat).Abs(area), big.NewRat(2, 1))
	if e := ValidateBoundary(ctx, rounded, o.Validation); e != nil {
		return empty, ErrConversion
	}
	// Remove only redundant collinear vertices for the polygon triangulator;
	// afterwards split triangle boundary edges to put every one back exactly.
	simplified := make([][]geom.Vec3, len(b.Loops))
	for li, l := range rounded.Loops {
		ids := append([]int(nil), out.LoopVertices[li]...)
		for changed := true; changed; {
			changed = false
			for i, id := range ids {
				if e := tick(); e != nil {
					return empty, e
				}
				a, c := ids[(i+len(ids)-1)%len(ids)], ids[(i+1)%len(ids)]
				crossing := cross(diff(vec(out.Mesh.Vertices[id]), vec(out.Mesh.Vertices[a])), diff(vec(out.Mesh.Vertices[c]), vec(out.Mesh.Vertices[a])))
				if crossing[b.Axis].Sign() == 0 {
					ids = append(ids[:i], ids[i+1:]...)
					changed = true
					break
				}
			}
		}
		for _, id := range ids {
			simplified[li] = append(simplified[li], out.Mesh.Vertices[id])
		}
		_ = l
	}
	direction := geom.Vec3{}
	switch b.Axis {
	case 0:
		direction.X = 1
	case 1:
		direction.Y = 1
	case 2:
		direction.Z = 1
	}
	for i, l := range b.Loops {
		if l.Hole {
			continue
		}
		var holes [][]geom.Vec3
		for j, h := range b.Loops {
			if h.Hole && h.Parent == i {
				holes = append(holes, simplified[j])
			}
		}
		prism, e := brep.PrismWithHoles(simplified[i], holes, direction)
		if e != nil {
			return empty, ErrConversion
		}
		m, faceMap := prism.Tessellate(), prism.TriangleFaces()
		for j, tr := range m.Triangles {
			if faceMap[j] != 0 {
				continue
			}
			var t [3]int
			for k, id := range tr {
				index, ok := seen[m.Vertices[id]]
				if !ok {
					return empty, ErrConversion
				}
				t[k] = index
			}
			if cross(diff(vec(out.Mesh.Vertices[t[1]]), vec(out.Mesh.Vertices[t[0]])), diff(vec(out.Mesh.Vertices[t[2]]), vec(out.Mesh.Vertices[t[0]])))[b.Axis].Sign() != normal[b.Axis].Sign() {
				t[1], t[2] = t[2], t[1]
			}
			out.Mesh.Triangles = append(out.Mesh.Triangles, t)
			if len(out.Mesh.Triangles) > o.MaxTriangles {
				return empty, ErrLimit
			}
		}
	}
	used := make([]bool, len(out.Mesh.Vertices))
	for _, t := range out.Mesh.Triangles {
		for _, id := range t {
			used[id] = true
		}
	}
	for id, p := range out.Mesh.Vertices {
		if used[id] {
			continue
		}
		inserted := false
		var tris [][3]int
		for _, t := range out.Mesh.Triangles {
			split := -1
			for j, a := range t {
				if e := tick(); e != nil {
					return empty, e
				}
				c := t[(j+1)%3]
				pa, pc := vec(out.Mesh.Vertices[a]), vec(out.Mesh.Vertices[c])
				d, q := diff(pc, pa), diff(vec(p), pa)
				if dot(cross(d, q), cross(d, q)).Sign() == 0 && dot(q, d).Sign() > 0 && dot(q, d).Cmp(dot(d, d)) < 0 {
					split = j
					break
				}
			}
			if split < 0 {
				tris = append(tris, t)
			} else {
				a, c, other := t[split], t[(split+1)%3], t[(split+2)%3]
				tris = append(tris, [3]int{a, id, other}, [3]int{id, c, other})
				inserted = true
			}
			if len(tris) > o.MaxTriangles {
				return empty, ErrLimit
			}
		}
		if !inserted {
			return empty, ErrConversion
		}
		out.Mesh.Triangles = tris
		used[id] = true
	}
	counts := map[[2]int]int{}
	sum := new(big.Rat)
	for _, t := range out.Mesh.Triangles {
		if e := tick(); e != nil {
			return empty, e
		}
		a, c, d := vec(out.Mesh.Vertices[t[0]]), vec(out.Mesh.Vertices[t[1]]), vec(out.Mesh.Vertices[t[2]])
		weight := cross(diff(c, a), diff(d, a))[b.Axis]
		if weight.Sign() != normal[b.Axis].Sign() {
			return empty, ErrConversion
		}
		sum.Add(sum, weight)
		for i, a := range t {
			c := t[(i+1)%3]
			counts[[2]int{a, c}]++
		}
	}
	expected := map[[2]int]bool{}
	for _, loop := range out.LoopVertices {
		for i, a := range loop {
			expected[[2]int{a, loop[(i+1)%len(loop)]}] = true
		}
	}
	for edge, count := range counts {
		if count != 1 {
			return empty, ErrConversion
		}
		opposite := counts[[2]int{edge[1], edge[0]}]
		if opposite == 0 {
			if !expected[edge] {
				return empty, ErrConversion
			}
			delete(expected, edge)
		} else if expected[edge] {
			return empty, ErrConversion
		}
	}
	if len(expected) != 0 || div(new(big.Rat).Abs(sum), big.NewRat(2, 1)).Cmp(rounded.ProjectedArea) != 0 {
		return empty, ErrConversion
	}
	if e := out.Mesh.Validate(); e != nil {
		return empty, ErrConversion
	}
	return out, nil
}

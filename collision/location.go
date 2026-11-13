package collision

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"sort"
)

// Location describes membership in the material bounded by a validated solid.
type Location uint8

const (
	Outside Location = iota
	Inside
	OnBoundary
)

// LocationOptions bounds preparation and each individual query separately.
// Zero defaults are 4096 triangles, 2000000 work units, 100000 collision pairs,
// and 64 ray directions. MaxRays may not exceed 1024. Collision validation is
// bounded separately and is not internally cancellable; other loops check ctx.
type LocationOptions struct{ MaxTriangles, MaxWork, MaxCollisionPairs, MaxRays int }

func locationDefaults(o LocationOptions) (LocationOptions, error) {
	ps := []*int{&o.MaxTriangles, &o.MaxWork, &o.MaxCollisionPairs, &o.MaxRays}
	ds := []int{4096, 2000000, 100000, 64}
	for i, p := range ps {
		if *p < 0 {
			return o, ErrInvalid
		}
		if *p == 0 {
			*p = ds[i]
		}
	}
	if o.MaxTriangles > MaxTriangles || o.MaxRays > 1024 {
		return o, ErrInvalid
	}
	return o, nil
}

// RayHit records one transverse crossing of the successful ray. Parameter is
// relative to the returned RayDirection; ExactPoint and Parameter are owned by
// the caller. Exiting is true when crossing out of material. Rounded Point is a
// convenience and may move off its exact face.
type RayHit struct {
	Face, Triangle int
	Parameter      *big.Rat
	ExactPoint     [3]*big.Rat
	Point          geom.Vec3
	Exiting        bool
}

// LocationResult gives stable source ancestry. For OnBoundary, Face and Triangle
// select the first containing triangle in input tessellation order; Hits is empty
// and Winding is undefined (zero). Otherwise Face/Triangle are -1, Hits is sorted
// along the ray, and Winding is zero outside or one inside. An empty solid has no
// ray direction or hits. RayAttempts exposes deterministic degeneracy retries.
type LocationResult struct {
	Location                Location
	Face, Triangle, Winding int
	RayDirection            geom.Vec3
	RayAttempts, Work       int
	Hits                    []RayHit
}
type locationTriangle struct {
	v    [3]point
	n    point
	d    scalar
	face int
}

// PointLocator is an immutable validated snapshot, safe for concurrent queries.
// Its zero value represents an empty boundary.
// Construction requires embedded closed manifold boundaries with outward outer
// shells, inward cavities, and alternating orientation at deeper nesting levels.
// Exact geometric self-intersections and touching distinct shells are rejected.
// A locator classifies the represented planar tessellation, not curved surfaces.
type PointLocator struct {
	triangles []locationTriangle
	shells    []ShellInfo
	options   LocationOptions
}
type locationBudget struct {
	ctx         context.Context
	limit, work int
}

func (b *locationBudget) tick(n int) error {
	if e := b.ctx.Err(); e != nil {
		return e
	}
	if n > b.limit-b.work {
		return ErrLimit
	}
	b.work += n
	return nil
}
func triangleMembership(p point, t locationTriangle) (inside, boundary bool) {
	for i, a := range t.v {
		sign := dot(cross(minus(t.v[(i+1)%3], a), minus(p, a)), t.n).Sign()
		if sign < 0 {
			return false, false
		}
		boundary = boundary || sign == 0
	}
	return true, boundary
}

// NewPointLocator validates an embedded material boundary once. Zero-value BRep
// solids are valid empty inputs. Validation failures return no locator.
func NewPointLocator(ctx context.Context, s brep.Solid, o LocationOptions) (*PointLocator, error) {
	if ctx == nil {
		return nil, ErrInvalid
	}
	var err error
	o, err = locationDefaults(o)
	if err != nil {
		return nil, err
	}
	b := &locationBudget{ctx: ctx, limit: o.MaxWork}
	if err = b.tick(1); err != nil {
		return nil, err
	}
	m := s.Tessellate()
	if len(m.Triangles) > o.MaxTriangles {
		return nil, ErrLimit
	}
	loc := &PointLocator{options: o}
	if len(m.Triangles) == 0 {
		return loc, nil
	}
	if err = m.ValidateSolid(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	report, err := SelfIntersections(m, Options{MaxPairs: o.MaxCollisionPairs, MaxContacts: 1})
	if len(report.Contacts) > 0 {
		return nil, fmt.Errorf("%w: boundary self-intersection or shell contact", ErrInvalid)
	}
	if err != nil {
		return nil, err
	}
	if !report.Complete {
		return nil, ErrLimit
	}
	if err = b.tick(1); err != nil {
		return nil, err
	}
	faces := s.TriangleFaces()
	loc.triangles = make([]locationTriangle, len(m.Triangles))
	adj := make([][]int, len(m.Triangles))
	edges := map[[2]int]int{}
	for i, tr := range m.Triangles {
		if err = b.tick(1); err != nil {
			return nil, err
		}
		t := locationTriangle{face: faces[i]}
		for j, k := range tr {
			t.v[j] = exact(m.Vertices[k])
		}
		t.n = triangleNormal(t.v)
		t.d = dot(t.n, t.v[0])
		loc.triangles[i] = t
		for j, v := range tr {
			w := tr[(j+1)%3]
			if v > w {
				v, w = w, v
			}
			key := [2]int{v, w}
			if other, ok := edges[key]; ok {
				adj[i] = append(adj[i], other)
				adj[other] = append(adj[other], i)
			} else {
				edges[key] = i
			}
		}
	}
	seen := make([]bool, len(adj))
	var shells [][]int
	for i := range adj {
		if seen[i] {
			continue
		}
		seen[i] = true
		todo := []int{i}
		var shell []int
		for len(todo) > 0 {
			if err = b.tick(1); err != nil {
				return nil, err
			}
			j := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			shell = append(shell, j)
			for _, k := range adj[j] {
				if !seen[k] {
					seen[k] = true
					todo = append(todo, k)
				}
			}
		}
		shells = append(shells, shell)
	}
	loc.shells = make([]ShellInfo, len(shells))
	containers := make([][]int, len(shells))
	for i, shell := range shells {
		origin := loc.triangles[shell[0]].v[0]
		volume := new(big.Rat)
		for _, j := range shell {
			if err = b.tick(1); err != nil {
				return nil, err
			}
			t := loc.triangles[j]
			volume = add(volume, dot(minus(t.v[0], origin), cross(minus(t.v[1], origin), minus(t.v[2], origin))))
		}
		depth := 0
		for j, other := range shells {
			if i == j {
				continue
			}
			hits, _, err := loc.ray(origin, other, b)
			if err != nil {
				return nil, err
			}
			if len(hits)%2 == 1 {
				depth++
				containers[i] = append(containers[i], j)
			}
		}
		expected := 1
		if depth%2 == 1 {
			expected = -1
		}
		loc.shells[i] = ShellInfo{Parent: -1, Depth: depth, Triangles: append([]int(nil), shell...), SignedVolume: new(big.Rat).Quo(volume, big.NewRat(6, 1))}
		if volume.Sign() != expected {
			return nil, fmt.Errorf("%w: shell orientation disagrees with material nesting", ErrInvalid)
		}
	}
	for i, parents := range containers {
		for _, j := range parents {
			p := loc.shells[i].Parent
			if p < 0 || loc.shells[j].Depth > loc.shells[p].Depth {
				loc.shells[i].Parent = j
			}
		}
	}
	return loc, nil
}

type locationCrossing struct {
	triangle int
	t        scalar
	p        point
	sign     int
}

func (l *PointLocator) ray(p point, indices []int, b *locationBudget) ([]locationCrossing, int, error) {
	for k := 1; k <= l.options.MaxRays; k++ {
		dir := point{big.NewRat(1, 1), big.NewRat(int64(k), 1), big.NewRat(int64(k*k), 1)}
		ambiguous := false
		var hits []locationCrossing
		count := len(l.triangles)
		if indices != nil {
			count = len(indices)
		}
		for j := 0; j < count; j++ {
			if err := b.tick(1); err != nil {
				return nil, 0, err
			}
			i := j
			if indices != nil {
				i = indices[j]
			}
			tr := l.triangles[i]
			den := dot(tr.n, dir)
			num := sub(tr.d, dot(tr.n, p))
			if den.Sign() == 0 {
				if num.Sign() == 0 {
					ambiguous = true
					break
				}
				continue
			}
			parameter := div(num, den)
			if parameter.Sign() <= 0 {
				continue
			}
			q := plus(p, scale(dir, parameter))
			inside, on := triangleMembership(q, tr)
			if !inside {
				continue
			}
			if on {
				ambiguous = true
				break
			}
			hits = append(hits, locationCrossing{i, parameter, q, den.Sign()})
		}
		if !ambiguous {
			sort.Slice(hits, func(i, j int) bool { return hits[i].t.Cmp(hits[j].t) < 0 })
			return hits, k, nil
		}
	}
	return nil, 0, fmt.Errorf("%w: all permitted ray directions are degenerate", ErrLimit)
}

// Locate classifies p exactly, without a distance tolerance. Points on edges or
// vertices are OnBoundary. A nonboundary ray hitting a triangle edge, vertex, or
// coplanar triangle plane is retried with direction (1,k,k*k), k=1..MaxRays.
// Exhausted directions return ErrLimit, never a guessed inside/outside result.
func (l *PointLocator) Locate(ctx context.Context, p geom.Vec3) (LocationResult, error) {
	empty := LocationResult{}
	if l == nil || ctx == nil || !p.IsFinite() {
		return empty, ErrInvalid
	}
	limit := l.options.MaxWork
	if limit == 0 {
		limit = 2000000
	}
	b := &locationBudget{ctx: ctx, limit: limit}
	if err := b.tick(1); err != nil {
		return empty, err
	}
	q := exact(p)
	r := LocationResult{Location: Outside, Face: -1, Triangle: -1}
	for i, t := range l.triangles {
		if err := b.tick(1); err != nil {
			return empty, err
		}
		if dot(t.n, q).Cmp(t.d) != 0 {
			continue
		}
		if inside, _ := triangleMembership(q, t); inside {
			r.Location, r.Face, r.Triangle, r.Work = OnBoundary, t.face, i, b.work
			return r, nil
		}
	}
	if len(l.triangles) == 0 {
		r.Work = b.work
		return r, nil
	}
	hits, k, err := l.ray(q, nil, b)
	if err != nil {
		return empty, err
	}
	r.RayDirection = geom.Vec3{X: 1, Y: float64(k), Z: float64(k * k)}
	r.RayAttempts = k
	for _, h := range hits {
		r.Winding += h.sign
		x := RayHit{Face: l.triangles[h.triangle].face, Triangle: h.triangle, Parameter: new(big.Rat).Set(h.t), Exiting: h.sign > 0}
		for i := 0; i < 3; i++ {
			x.ExactPoint[i] = new(big.Rat).Set(h.p[i])
		}
		x.Point = rounded([]point{h.p})[0]
		r.Hits = append(r.Hits, x)
	}
	if r.Winding != 0 && r.Winding != 1 {
		return empty, fmt.Errorf("%w: inconsistent winding", ErrInvalid)
	}
	if r.Winding == 1 {
		r.Location = Inside
	}
	r.Work = b.work
	return r, nil
}

// LocatePoint validates s and performs one query. Reuse NewPointLocator for
// repeated queries to avoid repeating geometric and shell-orientation validation.
func LocatePoint(ctx context.Context, s brep.Solid, p geom.Vec3, o LocationOptions) (LocationResult, error) {
	l, err := NewPointLocator(ctx, s, o)
	if err != nil {
		return LocationResult{}, err
	}
	return l.Locate(ctx, p)
}

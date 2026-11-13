package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedClosedAlgebraicTubeLens(t *testing.T) {
	ctx := context.Background()
	source := algebraicRunMeshFixture(t)
	lens, e := source.SealAlgebraicTubeLens(ctx, AlgebraicTubeLensOptions{})
	if e != nil {
		t.Fatal(e)
	}
	options := AlgebraicTubeLensMeshOptions{Boundary: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .04}}}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .02}}
	m, e := lens.TessellateClosedCertified(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	if e := m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if !m.Collision.Complete || len(m.Collision.Contacts) > 0 {
		t.Fatal("contacts")
	}
	v, _ := m.SignedDisplayVolume.Float64()
	r := 5.0 / 16
	want := math.Pi * r * r * r * r / 2
	if math.Abs(v-want) > .15*want {
		t.Fatal(v, want)
	}
	for id, local := range m.BoundaryVertices {
		if m.Mesh.Vertices[local] != m.Boundary.Run.Vertices[id] {
			t.Fatal("changed boundary")
		}
	}
	edges := map[[2]int]bool{}
	owners := map[[2]int]map[int]bool{}
	for i, tri := range m.Mesh.Triangles {
		for j, a := range tri {
			b := tri[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			key := [2]int{a, b}
			edges[key] = true
			if owners[key] == nil {
				owners[key] = map[int]bool{}
			}
			owners[key][m.OriginalFace[i]] = true
		}
	}
	if len(m.Mesh.Vertices)-len(edges)+len(m.Mesh.Triangles) != 2 {
		t.Fatal("Euler")
	}
	for _, path := range m.Boundary.Run.PieceVertices {
		for j := 0; j+1 < len(path); j++ {
			a, b := m.BoundaryVertices[path[j]], m.BoundaryVertices[path[j+1]]
			if a > b {
				a, b = b, a
			}
			if len(owners[[2]int{a, b}]) != 2 {
				t.Fatal("crease ownership")
			}
		}
	}
	if m.SourceImageSquaredDistance.Sign() <= 0 {
		t.Fatal("missing source proof")
	}

	bound, _ := m.SourceImageSquaredDistance.Float64()
	surface := func(x, y float64, face int) geom.Vec3 {
		z := x * y / 64
		if face == 1 {
			z += (x-.5)*(x-.5) + (y-.5)*(y-.5) - r*r
		}
		return geom.Vec3{X: x, Y: y, Z: z}
	}
	// Independent source->display check by vertical intersection with the
	// represented triangle projection; all samples are safely inside the disk.
	for face := 0; face < 2; face++ {
		for _, radius := range []float64{0, r * .5, r * .9} {
			for i := 0; i < 16; i++ {
				angle := 2 * math.Pi * float64(i) / 16
				p := surface(.5+radius*math.Cos(angle), .5+radius*math.Sin(angle), face)
				best := math.Inf(1)
				for j, tri := range m.Mesh.Triangles {
					if m.OriginalFace[j] != face {
						continue
					}
					a, b, c := m.Mesh.Vertices[tri[0]], m.Mesh.Vertices[tri[1]], m.Mesh.Vertices[tri[2]]
					d := (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
					u := ((p.X-a.X)*(c.Y-a.Y) - (p.Y-a.Y)*(c.X-a.X)) / d
					v := ((b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X)) / d
					if u >= -1e-12 && v >= -1e-12 && u+v <= 1+1e-12 {
						q := a.Scale(1 - u - v).Add(b.Scale(u)).Add(c.Scale(v))
						distance := p.Sub(q).Dot(p.Sub(q))
						if distance < best {
							best = distance
						}
					}
				}
				if best > bound+1e-12 {
					t.Fatal("source/display bound", best, bound)
				}
			}
		}
	}
	// Independent display->source witness: nearest XY disk point lies on
	// the original graph, including a centroid outside its circular footprint.
	for i, tri := range m.Mesh.Triangles {
		p := m.Mesh.Vertices[tri[0]].Add(m.Mesh.Vertices[tri[1]]).Add(m.Mesh.Vertices[tri[2]]).Scale(1.0 / 3)
		x, y := p.X-.5, p.Y-.5
		rho := math.Hypot(x, y)
		if rho > r {
			x *= r / rho
			y *= r / rho
		}
		q := surface(x+.5, y+.5, m.OriginalFace[i])
		if p.Sub(q).Dot(p.Sub(q)) > bound+1e-12 {
			t.Fatal("display/source bound")
		}
	}
	t.Log(len(m.Mesh.Vertices), len(m.Mesh.Triangles), v, want, m.TriangleChecks)
	options.Interior.MaxTriangles = 1
	if m, e := lens.TessellateClosedCertified(ctx, options); e == nil || len(m.Mesh.Vertices) > 0 {
		t.Fatal("partial budget result", e)
	}
}

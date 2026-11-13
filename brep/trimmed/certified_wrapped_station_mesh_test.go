package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedWrappedStationSolid(t *testing.T) {
	ctx := context.Background()
	stations := []WrappedLoftStation{{Z: -2, Radius: 1}, {Z: -.5, Radius: 2}, {Z: 1, Radius: .75}, {Z: 3, Radius: 1.5}}
	s, h, e := LoftWrappedCircularStations(ctx, geom.Vec2{X: .125, Y: -.25}, stations, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}

	var native bytes.Buffer
	if e := WriteJSON(&native, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSONContext(ctx, bytes.NewReader(native.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	o := CertifiedWrappedStationMeshOptions{Band: CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .08}}}
	m, e := s.TessellateWrappedStationLoftCertified(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if e := m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
		t.Fatal("embedding")
	}
	if len(m.Source.History.StationRims) != len(stations) || len(m.StationVertices) != len(stations) {
		t.Fatal("station ancestry")
	}
	owners := map[[2]int]map[int]bool{}
	for i, tri := range m.Mesh.Triangles {
		for j, a := range tri {
			b := tri[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			key := [2]int{a, b}
			if owners[key] == nil {
				owners[key] = map[int]bool{}
			}
			owners[key][m.OriginalFace[i]] = true
		}
	}
	if len(m.Mesh.Vertices)-len(owners)+len(m.Mesh.Triangles) != 2 {
		t.Fatal("Euler")
	}
	for station, ring := range m.StationVertices {
		want := map[int]bool{}
		if station == 0 {
			want[h.CapFaces[0]] = true
			want[h.WallFaces[0]] = true
		} else if station == len(stations)-1 {
			want[h.CapFaces[1]] = true
			want[h.WallFaces[station-1]] = true
		} else {
			want[h.WallFaces[station-1]] = true
			want[h.WallFaces[station]] = true
		}
		for i, a := range ring {
			b := ring[(i+1)%len(ring)]
			if a > b {
				a, b = b, a
			}
			got := owners[[2]int{a, b}]
			if len(got) != 2 {
				t.Fatal("crease incidence")
			}
			for face := range want {
				if !got[face] {
					t.Fatal("original face ancestry")
				}
			}
			p := m.Mesh.Vertices[ring[i]]
			if p.Z != stations[station].Z {
				t.Fatal("station moved")
			}
		}
	}
	want := 0.0
	for i, a := range stations[:len(stations)-1] {
		b := stations[i+1]
		want += math.Pi * (b.Z - a.Z) * (a.Radius*a.Radius + a.Radius*b.Radius + b.Radius*b.Radius) / 3
	}
	got, _ := m.SignedDisplayVolume.Float64()
	if math.Abs(got-want) > .02*want {
		t.Fatal(got, want)
	}
	t.Log(len(m.Mesh.Vertices), len(m.Mesh.Triangles), m.Subdivisions, m.TriangleChecks, m.StationSamples, got, want)
	o.MaxStationSamples = 1
	bad, e := s.TessellateWrappedStationLoftCertified(ctx, o)
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("station budget partial", e)
	}
}

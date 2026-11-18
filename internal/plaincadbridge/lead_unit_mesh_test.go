package plaincadbridge

import (
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/internal/slowtest"
	"testing"
	"time"
)

func TestOriginalLeadUnitMeshes(t *testing.T) {
	slowtest.Skip(t)
	for _, starts := range []int{1, 2, 4} {
		t.Run(string(rune('0'+starts)), func(t *testing.T) {
			hand := 1
			slope := math.Tan(math.Pi / 12)
			if starts == 2 {
				hand = -1
				slope = math.Tan(14.5 * math.Pi / 180)
			}
			d := leadRequest{Major: 4, Minor: 3, Bore: 3.1, NutOuter: 6, Pitch: 2, Lead: number(2 * starts), ScrewLength: 18, NutLength: 10, Clearance: .1, Slope: number(slope), Starts: starts, Hand: hand}
			for _, nut := range []bool{true, false} {
				start := time.Now()
				b := New()
				shape, e := b.createLeadPart(d, nut)
				if e != nil {
					t.Fatal(nut, e)
				}
				t.Log("constructed", nut, time.Since(start))
				m, e := b.shapes[shape.Handle].tessellate(request{LinearDeflection: optionalNumber{value: .05, set: true}, AngularDeflection: optionalNumber{value: .1, set: true}})
				if e != nil {
					t.Fatal("mesh", nut, e, time.Since(start))
				}
				if e = m.ValidateSolid(); e != nil {
					t.Fatal(e)
				}
				for i, p := range m.Vertices {
					m.Vertices[i].X = float64(float32(p.X))
					m.Vertices[i].Y = float64(float32(p.Y))
					m.Vertices[i].Z = float64(float32(p.Z))
				}
				report, e := collision.SelfIntersectionsBVHContext(context.Background(), m, collision.Options{MaxPairs: 10000000, MaxContacts: 1})
				if e != nil || !report.Complete || len(report.Contacts) > 0 {
					t.Fatal("float32 embedding", nut, e, report)
				}
				t.Log("mesh", nut, len(m.Triangles), time.Since(start))
			}
		})
	}
}

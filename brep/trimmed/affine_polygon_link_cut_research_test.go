//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/intersect"
	"testing"
)

func polygonLinkCutFixture(t *testing.T) (*ConvexAffinePolygonCrossing, ConvexAffinePolygonLinkCutOptions) {
	t.Helper()
	a, b := affinePolygonPairedFixture(t, true)
	c, e := CertifyConvexAffinePolygonCrossing(context.Background(), a, b, ConvexAffinePolygonCrossingOptions{AllowPairedEdgeJunctions: true})
	if e != nil {
		t.Fatal(e)
	}
	return c, ConvexAffinePolygonLinkCutOptions{Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}}
}

func TestConvexAffinePolygonLinkCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	c, o := polygonLinkCutFixture(t)
	r, e := c.CutWithSeparatedVertexLinks(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	s := r.Solid()
	if len(s.vertices) != 23 || len(s.edges) != 63 || len(s.faces) != 42 || len(s.vertices)-len(s.edges)+len(s.faces) != 2 || r.CompleteFacePairs() != 861 {
		t.Fatal("incomplete topology")
	}
	want := new(big.Rat).Sub(big.NewRat(255, 8), affineMaterialFixtureCommonVolume())
	if r.ExactVolume().Cmp(want) != 0 {
		t.Fatal(r.ExactVolume())
	}
	contacts := r.IsolatedVertexContacts()
	if len(contacts) != 1 || len(contacts[0].Vertices) != 2 {
		t.Fatal("contact ledger")
	}
	contact := contacts[0]
	node := c.SectionVertices()[contact.SectionVertex]
	if len(node.Owners) != 2 || len(node.ContactPairs) != 4 {
		t.Fatal("unbound original source")
	}
	for k := range contact.Point {
		if contact.Point[k].Cmp(node.Point[k]) != 0 {
			t.Fatal("wrong source point")
		}
	}
	if contact.Point[0].Sign() != 0 || contact.Point[1].Cmp(big.NewRat(-1, 1)) != 0 || contact.Point[2].Sign() != 0 {
		t.Fatal("wrong pinch")
	}
	if contact.Vertices[0] == contact.Vertices[1] || s.vertices[contact.Vertices[0]] != s.vertices[contact.Vertices[1]] {
		t.Fatal("vertex identities")
	}
	if len(contact.IncidentFaces[0]) != 4 || len(contact.IncidentFaces[1]) != 6 {
		t.Fatal("fan partition")
	}
	if !affinePolygonConnectedFaces(s) {
		t.Fatal("not one connected shell")
	}
	if e = affineMaterialVertexLinks(ctx, s); e != nil {
		t.Fatal(e)
	}
	retained := 0
	for _, h := range r.Fragments() {
		if h.Retained {
			retained++
		}
	}
	if retained != len(s.faces) {
		t.Fatal("lost material")
	}
	for fi, h := range r.FaceSources() {
		original := c.source.sources[h.Owner].faces[h.Face]
		if s.faces[fi].Surface != original.Surface || s.faces[fi].Reversed != (original.Reversed != h.Reversed) || h.Reversed != (h.Owner == 1) || r.SourceFaceFragments(h.Owner, h.Face) == nil {
			t.Fatal("original ownership")
		}
		ids := polygonFaceVertexIDs(s, s.faces[fi])
		if len(ids) != 3 {
			t.Fatal("triangle")
		}
		p := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		for _, id := range ids {
			q := affineMaterialXYZ(s.vertices[id])
			for k := range p {
				p[k].Add(p[k], new(big.Rat).Quo(q[k], big.NewRat(3, 1)))
			}
		}
		if h.Owner == 0 {
			// Independent exact inverse of the authored tool map.
			w := new(big.Rat).Mul(big.NewRat(4, 3), new(big.Rat).Sub(p[2], p[0]))
			rawS := new(big.Rat).Sub(p[0], new(big.Rat).Quo(w, big.NewRat(4, 1)))
			rawT := new(big.Rat).Add(new(big.Rat).Sub(p[1], new(big.Rat).Quo(rawS, big.NewRat(2, 1))), new(big.Rat).Quo(w, big.NewRat(8, 1)))
			outside := rawS.Cmp(big.NewRat(-1, 1)) < 0 || rawS.Cmp(big.NewRat(1, 1)) > 0 || rawT.Cmp(big.NewRat(-1, 1)) < 0 || rawT.Cmp(big.NewRat(1, 1)) > 0 || w.Cmp(big.NewRat(-2, 1)) < 0 || w.Sign() > 0
			if !outside || h.InsideOther {
				t.Fatal("wrong stock occupancy")
			}
		} else {
			inside := p[0].Cmp(big.NewRat(-2, 1)) > 0 && p[0].Cmp(big.NewRat(2, 1)) < 0 && p[1].Cmp(big.NewRat(-1, 1)) > 0 && p[1].Cmp(big.NewRat(1, 1)) < 0 && p[2].Cmp(big.NewRat(-4, 1)) > 0 && p[2].Sign() < 0 && new(big.Rat).Sub(p[1], p[0]).Cmp(big.NewRat(11, 4)) < 0
			if !inside || !h.InsideOther {
				t.Fatal("wrong cutter occupancy")
			}
		}
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, s); e != nil {
		t.Fatal(e)
	}
	restored, e := ReadJSONContext(ctx, &buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(restored.vertices) != len(s.vertices) || len(restored.edges) != len(s.edges) {
		t.Fatal("identities lost")
	}
	if e = affineMaterialVertexLinks(ctx, restored); e != nil {
		t.Fatal(e)
	}
	if restored.vertices[contact.Vertices[0]] != restored.vertices[contact.Vertices[1]] {
		t.Fatal("point altered")
	}
	restoredContacts, e := c.polygonCoincidentVertices(ctx, restored, 1)
	if e != nil || len(restoredContacts) != 1 {
		t.Fatal("reload contacts", e)
	}
	budget := &nativePrismAffineBudget{ctx: ctx, max: 64000000, bits: 16384}
	if e = c.verifyPolygonMaterialSolid(ctx, budget, restored, r.FaceSources(), o.Material); e != nil {
		t.Fatal("reload original source agreement", e)
	}
	if n, e := c.verifyPolygonContactTopology(ctx, budget, restored, r.FaceSources(), restoredContacts); e != nil || n != 861 {
		t.Fatal("reload full contact reproof", e)
	}
	contacts[0].Point[0].SetInt64(99)
	contacts[0].Vertices[0] = -1
	contacts[0].IncidentFaces[0][0] = -1
	if r.IsolatedVertexContacts()[0].Point[0].Sign() != 0 || r.IsolatedVertexContacts()[0].Vertices[0] < 0 || r.IsolatedVertexContacts()[0].IncidentFaces[0][0] < 0 {
		t.Fatal("mutable certificate")
	}
	exact := o
	exact.Material.MaxOperations = r.ExactOperations()
	if _, e = c.CutWithSeparatedVertexLinks(ctx, exact); e != nil {
		t.Fatal("exact quota", e)
	}
	exact.Material.MaxOperations--
	if out, e := c.CutWithSeparatedVertexLinks(ctx, exact); out != nil || e == nil {
		t.Fatal("partial quota output")
	}
	if out, e := c.Combine(ctx, intersect.AffineRingDifference, o.Material); out != nil || e == nil {
		t.Fatal("strict path changed")
	}
	t.Log("exact material", r.ExactVolume(), "work", r.ExactOperations(), "pairs", r.CompleteFacePairs())
}

func TestConvexAffinePolygonLinkCutRejects(t *testing.T) {
	ctx := context.Background()
	c, o := polygonLinkCutFixture(t)
	for _, bad := range []ConvexAffinePolygonLinkCutOptions{{Material: o.Material, MaxFacePairs: 860}, {Material: o.Material, MaxContactPoints: 65}, {Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true, MaxOperations: 1}}, {Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true, MaxFaces: 41}}, {Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true, Sew: SewOptions{MaxUses: 125}}}} {
		if r, e := c.CutWithSeparatedVertexLinks(ctx, bad); r != nil || e == nil {
			t.Fatal("non-atomic limit")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := c.CutWithSeparatedVertexLinks(canceled, o); r != nil || e == nil {
		t.Fatal("cancellation")
	}
	damaged := *c
	damaged.nodes = nil
	if r, e := damaged.CutWithSeparatedVertexLinks(ctx, o); r != nil || e == nil {
		t.Fatal("unowned original contact")
	}
	r, e := c.CutWithSeparatedVertexLinks(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	budget := func() *nativePrismAffineBudget { return &nativePrismAffineBudget{ctx: ctx, max: 64000000, bits: 16384} }
	if _, e = c.verifyPolygonContactTopology(ctx, budget(), r.Solid(), r.FaceSources(), nil); e == nil {
		t.Fatal("omitted physical contact")
	}
	proof := r.IsolatedVertexContacts()
	proof[0].SectionVertex++
	if _, e = c.verifyPolygonContactTopology(ctx, budget(), r.Solid(), r.FaceSources(), proof); e == nil {
		t.Fatal("stale source owner")
	}
	proof = r.IsolatedVertexContacts()
	proof[0].Point[0].SetInt64(1)
	if _, e = c.verifyPolygonContactTopology(ctx, budget(), r.Solid(), r.FaceSources(), proof); e == nil {
		t.Fatal("changed contact point")
	}
	_, faces, options, e := c.selectPolygonMaterial(ctx, intersect.AffineRingDifference, o.Material)
	if e != nil {
		t.Fatal(e)
	}
	edge := faces[0].Edges[0]
	a, z := faces[0].Vertices[edge.Start], faces[0].Vertices[edge.End]
	middle := geom.Vec3{X: (a.X + z.X) / 2, Y: (a.Y + z.Y) / 2, Z: (a.Z+z.Z)/2 + 1.0/(1<<40)}
	curve, e := geom.NewBezier([]geom.Vec3{a, middle, z}, nil)
	if e != nil {
		t.Fatal(e)
	}
	faces[0].Edges[0].Curve = curve
	faces[0].Edges[0].Range = parametric.Interval{Max: 1}
	if _, e = sewPolygonEdgeOccurrences(ctx, faces, options.Sew); e == nil {
		t.Fatal("endpoint-only edge agreement")
	}
	aSolid, bSolid := affinePolygonPairedFixture(t, true)
	reverse, e := CertifyConvexAffinePolygonCrossing(ctx, bSolid, aSolid, ConvexAffinePolygonCrossingOptions{AllowPairedEdgeJunctions: true})
	if e != nil {
		t.Fatal(e)
	}
	if r, e := reverse.CutWithSeparatedVertexLinks(ctx, o); r != nil || e == nil {
		t.Fatal("disconnected output hidden")
	}
}

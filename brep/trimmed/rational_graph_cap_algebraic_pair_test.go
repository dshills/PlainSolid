package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestAlgebraicGraphCapPairIrrationalOwners(t *testing.T) {
	ctx := context.Background()
	stock := makeWarpedSquareRationalGraphTube(t, 1./8)
	// Both individually rational sections cross only vertical stock
	// walls. Their seam Y=11/8, Z=1/2 meets the original sheared
	// through-void curve Y=1+X²/8 at the shared irrational X=sqrt(3).
	const z = .5 - 11./64
	planes := [2][3]geom.Vec3{
		{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}},
		{{Z: z}, {X: 1, Z: z}, {Y: 1, Z: z + 1./8}},
	}
	section := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	r, e := stock.ClipAlgebraicGraphCapPair(ctx, planes, section, RationalGraphShellRegionOptions{})
	if e != nil || len(r.Seams) != 2 || len(r.Caps[0].Wires) != 1 || len(r.Caps[1].Wires) != 1 {
		t.Fatal("expected two original material intervals shared by both caps", len(r.Seams), len(r.Caps[0].Wires), len(r.Caps[1].Wires), e)
	}
	opts := geom.ExactCurvePlaneEventOptions{}
	count := 0
	for _, seam := range r.Seams {
		for j := 0; j < 2; j++ {
			var pair [2]*geom.ExactComposedCurveEvent
			for i, owner := range seam.Owners {
				us := r.Caps[i].Wires[owner.Wire][owner.Use]
				k := j ^ i
				if us.Reversed {
					k ^= 1
				}
				root, e := us.Cell.EndpointRoot(k)
				if e != nil {
					t.Fatal(e)
				}
				pair[i], e = us.Cell.BindRoot(ctx, root, opts)
				if e != nil {
					t.Fatal(e)
				}
			}
			equal, _, e := pair[0].CompareModelPoint(ctx, pair[1], opts)
			if e != nil || !equal {
				t.Fatal("opposing coedges must agree at both exact endpoints", equal, e)
			}
			x, _, e := pair[0].CoordinateRoot(ctx, 0, opts)
			if e != nil {
				t.Fatal(e)
			}
			if x, e := x.Classify(ctx, opts); e != nil {
				t.Fatal(e)
			} else if x.Kind == geom.ExactRootKindIrrational {
				count++
			}
		}
	}
	attachments := 0
	for _, cap := range r.Caps {
		for wi, wire := range cap.Wires {
			for ui, edge := range wire {
				att, e := cap.OriginalStockAttachment(ctx, wi, ui, geom.SurfaceCurveAgreementOptions{}, opts)
				if edge.Seam >= 0 {
					if !errors.Is(e, geom.ErrInvalid) || att.Model != nil {
						t.Fatal("a generated seam has no source stock attachment", e)
					}
					continue
				}
				if e != nil || att.Source != edge.Source || att.StockReversed == edge.Reversed || att.ExactOperations <= 0 {
					t.Fatal("retained cap edge must bind to its exact original stock chart", wi, ui, att, e)
				}
				// Model and UV restrictions must keep the actual cap root,
				// which is irrational on the inner through-void terminal.
				for j := 0; j < 2; j++ {
					capRoot, _ := edge.Cell.EndpointRoot(j)
					uvRoot, _ := att.UV.EndpointRoot(j)
					order, e := capRoot.Compare(ctx, uvRoot, opts)
					if e != nil || !order.Proven || order.Order != 0 {
						t.Fatal("original stock parameter restriction lost its root", e)
					}
				}
				attachments++
			}
		}
	}
	if attachments < 12 {
		t.Fatal("missing retained stock owners", attachments)
	}
	stockLoops, e := r.OriginalStockLoops(ctx, section, opts)
	if e != nil || len(stockLoops) < 12 {
		t.Fatal("retained original chart loops must close without rounding", len(stockLoops), e)
	}
	owners := map[[3]int]bool{}
	for _, w := range stockLoops {
		if len(w.Uses) < 3 || w.OriginalFace < 0 || w.OriginalFace >= len(stock.faces) {
			t.Fatal("invalid original face loop", w)
		}
		for _, use := range w.Uses {
			if use.Cap >= 0 {
				id := [3]int{use.Cap, use.Owner.Wire, use.Owner.Use}
				if owners[id] || r.Caps[use.Cap].Wires[use.Owner.Wire][use.Owner.Use].Source.Face != w.OriginalFace {
					t.Fatal("duplicated or transferred original attachment", w.OriginalFace, id)
				}
				owners[id] = true
			}
		}
	}
	indexed, e := r.ExactIndexedBoundary(ctx, section, opts)
	if e != nil || indexed.BoundaryComponents != 1 || indexed.EulerCharacteristic != 2 ||
		indexed.Vertices != 24 || len(indexed.Edges) != 36 || len(indexed.Faces) != 14 || len(indexed.Faces) != len(stockLoops)+2 {
		t.Fatal("the cut opens the through-hole into an exterior notch, leaving one genus-zero shell", indexed.Vertices, len(indexed.Edges), len(indexed.Faces), indexed.EulerCharacteristic, e)
	}
	// The representation must ignore abandoned public-facing diagnostics.
	diag := r
	diag.Caps = [2]AlgebraicGraphCapClip{}
	diag.Seams = nil
	rechecked, e := diag.ExactIndexedBoundary(ctx, section, opts)
	if e != nil || rechecked.Vertices != indexed.Vertices || rechecked.EulerCharacteristic != indexed.EulerCharacteristic {
		t.Fatal("diagnostic edits changed the exact shell", e)
	}
	var data bytes.Buffer
	if e := WriteAlgebraicGraphStockJSON(&data, diag); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), data.Bytes()...)
	restored, topology, e := ReadAlgebraicGraphStockJSON(ctx, bytes.NewReader(raw), JSONLimits{},
		RationalGraphShellSectionOptions{}, RationalGraphShellRegionOptions{}, opts)
	if e != nil || !reflect.DeepEqual(topology, indexed) || len(restored.Seams) != 2 {
		t.Fatal("native reload must re-prove the exact closed indexed stock shell", e)
	}
	for i, cap := range r.Caps {
		for j, root := range cap.Section.Contacts {
			other := restored.Caps[i].Section.Contacts[j]
			order, e := root.Parameter.Compare(ctx, other.Parameter, opts)
			if e != nil || !order.Proven || order.Order != 0 || root.Source != other.Source {
				t.Fatal("saved stock recipe changed the exact junction", i, j, e)
			}
		}
	}
	if pair, boundary, e := ReadAlgebraicGraphStockJSON(ctx, bytes.NewReader(raw), JSONLimits{MaxBytes: 32},
		section, RationalGraphShellRegionOptions{}, opts); !errors.Is(e, geom.ErrConvergence) || len(pair.Seams)+len(boundary.Edges) != 0 {
		t.Fatal("bounded reader must fail before publishing", e)
	}
	// Independently integrate (Y−11/8)/8 over the sheared stock footprint
	// above the cut, then subtract the sheared void. Its lower edge turns
	// on at x=√3. This gives 697/960 − 3√3/640.
	width := big.NewRat(1, 1<<30)
	volume, e := restored.BoundVerticalPolynomialVolume(ctx, width, section, opts)
	ref := big.NewRat(697, 960)
	if e != nil || new(big.Rat).Sub(volume.Upper, volume.Lower).Cmp(width) > 0 {
		t.Fatal("clipped sheared shell volume proof", volume, e)
	}
	a, b := new(big.Rat).Sub(ref, volume.Upper), new(big.Rat).Sub(ref, volume.Lower)
	wantSquared := big.NewRat(27, 409600)
	if a.Sign() <= 0 || a.Mul(a, a).Cmp(wantSquared) > 0 || b.Mul(b, b).Cmp(wantSquared) < 0 {
		t.Fatal("certified volume must contain the independent irrational answer", volume)
	}
	wide, e := restored.BoundVerticalVolume(ctx, width, section, opts)
	if e != nil || wide.Lower.Cmp(volume.Lower) != 0 || wide.Upper.Cmp(volume.Upper) != 0 {
		t.Fatal("rational-cap method must retain the exact polynomial volume path", e)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ALGEBRAIC_CAP_STOCK"); oracle != "" {
		output, e := exec.CommandContext(ctx, oracle, "sheared-sleeve").Output()
		if e != nil {
			t.Fatal("pinned OCCT sheared sleeve", e)
		}
		var data [8]float64
		if e := json.Unmarshal(output, &data); e != nil {
			t.Fatal(e)
		}
		mid, _ := new(big.Rat).Quo(new(big.Rat).Add(volume.Lower, volume.Upper), big.NewRat(2, 1)).Float64()
		// OCCT keeps the prismatic faces whole; PlainSolid keeps the
		// original rectangular chart partitions. Their Euler number agrees.
		if math.Abs(data[0]-mid) > 2e-8 || int(data[5]-data[6]+data[7]) != topology.EulerCharacteristic {
			t.Fatal("independent OCCT volume or closed topology", data, mid)
		}
	}
	evaluated, curves, e := restored.ExactModelBoundary(ctx, section, opts)
	if e != nil || len(curves) != len(topology.Edges) || !reflect.DeepEqual(evaluated, topology) {
		t.Fatal("canonical model geometry for every indexed edge", len(curves), e)
	}
	meet := make(map[int]*geom.ExactComposedCurveEvent)
	for i, curve := range curves {
		for j, vertex := range [...]int{topology.Edges[i].Start, topology.Edges[i].End} {
			canonical := j
			if curve.Reversed {
				canonical ^= 1
			}
			cell := curve.Cells.Cells[canonical*(len(curve.Cells.Cells)-1)]
			root, e := cell.EndpointRoot(canonical)
			if e != nil {
				t.Fatal(e)
			}
			event, e := cell.BindRoot(ctx, root, opts)
			if e != nil {
				t.Fatal(e)
			}
			if old := meet[vertex]; old != nil {
				equal, _, e := old.CompareModelPoint(ctx, event, opts)
				if e != nil || !equal {
					t.Fatal("indexed model vertices must be exactly coincident", vertex, i, e)
				}
			} else {
				meet[vertex] = event
			}
		}
	}
	display := AlgebraicGraphStockBoundaryOptions{Section: section}
	display.Display.Deflection = .003
	network, e := restored.TessellateStockBoundaryCertified(ctx, display)
	if e != nil || len(network.Edges) != len(topology.Edges) || network.PairChecks <= 0 ||
		network.MaximumBoundaryError <= 0 || network.MaximumBoundaryError > display.Display.Deflection {
		t.Fatal("complete shared stock display", network.MaximumBoundaryError, network.PairChecks, e)
	}
	for i, path := range network.Edges {
		if path[0] != topology.Edges[i].Start || path[len(path)-1] != topology.Edges[i].End || !network.Curves[i].Complete {
			t.Fatal("shared chord ledger lost original indexed vertex", i)
		}
	}
	display.MaxPairs = 1
	if result, e := restored.TessellateStockBoundaryCertified(ctx, display); !errors.Is(e, geom.ErrConvergence) || len(result.Edges) != 0 {
		t.Fatal("display contact budget must fail atomically", e)
	}
	fill := AlgebraicGraphStockFillOptions{Deflection: .09}
	fill.Boundary.Section = section
	fill.Boundary.Display.Deflection = .003
	fill.Boundary.EndpointTolerance = 1e-12
	mesh, e := restored.TessellateAffineProjectedStockCertified(ctx, fill)
	if e != nil || mesh.SignedVolume == nil || len(mesh.Mesh.Triangles) != len(mesh.IndexedFaces) ||
		mesh.MaximumFillError <= 0 || mesh.MaximumFillError > fill.Deflection {
		t.Fatal("closed curved stock source-fill certificate", mesh.MaximumFillError, e)
	}
	mv, _ := mesh.SignedVolume.Float64()
	if math.Abs(mv-(697./960-3*math.Sqrt(3)/640)) > .025 {
		t.Fatal("independent display material volume", mv)
	}
	lineAnchor := [3]*big.Rat{new(big.Rat), big.NewRat(89, 64), big.NewRat(1, 2)}
	lineDirection := [3]*big.Rat{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}
	window := [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(4, 1)}
	line, e := restored.Caps[0].IntersectRationalLine(ctx, lineAnchor, lineDirection, window, opts)
	if e != nil || len(line.Contacts) != 4 || len(line.Material) != 2 {
		t.Fatal("in-plane line must preserve both material intervals and the void gap", len(line.Contacts), line.Material, e)
	}
	// Along y=89/64 the sheared void exits at x=5√2/4; the
	// other three original contacts are x=0, x=1 and x=3.
	for i, hit := range line.Contacts {
		q, e := hit.Parameter.Classify(ctx, opts)
		if e != nil || hit.Entry != (i%2 == 0) || hit.CurveParameter == nil {
			t.Fatal("wrong exact contact, owner parameter or material transition", i, hit, q, e)
		}
		if i == 2 {
			if q.Kind != geom.ExactRootKindIrrational {
				t.Fatal("lost the original irrational void intersection", q)
			}
			sign, e := hit.Parameter.SignPolynomial(ctx, []*big.Rat{big.NewRat(-25, 8), new(big.Rat), big.NewRat(1, 1)}, opts)
			if e != nil || sign.Sign != 0 {
				t.Fatal("void root must satisfy x²=25/8 exactly", sign, e)
			}
		} else if q.Kind != geom.ExactRootKindRational || q.Rational.Cmp(big.NewRat(int64(i), 1)) != 0 {
			t.Fatal("wrong rational side contact", i, q)
		}
	}
	coincident := [3]*big.Rat{new(big.Rat), big.NewRat(11, 8), big.NewRat(1, 2)}
	if partial, e := restored.Caps[0].IntersectRationalLine(ctx, coincident, lineDirection, window, opts); !errors.Is(e, geom.ErrConvergence) || len(partial.Contacts)+len(partial.Material) != 0 {
		t.Fatal("line coincident with true cap seams must not publish isolated events", partial, e)
	}
	lineDirection[0] = big.NewRat(-1, 1)
	lineAnchor[0] = big.NewRat(3, 1)
	window = [2]*big.Rat{big.NewRat(1, 4), big.NewRat(1, 2)}
	short, e := restored.Caps[0].IntersectRationalLine(ctx, lineAnchor, lineDirection, window, opts)
	if e != nil || len(short.Contacts) != 0 || len(short.Material) != 1 {
		t.Fatal("finite window entirely inside material", short, e)
	}
	for i, r := range short.Material[0] {
		q, e := r.Classify(ctx, opts)
		if e != nil || q.Kind != geom.ExactRootKindRational || q.Rational.Cmp(window[i]) != 0 {
			t.Fatal("material interval must preserve caller's exact bounds", i, q, e)
		}
	}
	fiberModel, e := restored.PrepareVerticalMaterial(ctx, section, opts)
	if e != nil {
		t.Fatal("vertical material proof", e)
	}
	q := big.NewRat
	axis := [3]*big.Rat{q(1, 1), q(0, 1), q(0, 1)}
	lineExtent := [2]*big.Rat{q(-1, 1), q(4, 1)}
	cross, e := fiberModel.IntersectRationalLine(ctx, [3]*big.Rat{q(0, 1), q(89, 64), q(513, 1024)}, axis, lineExtent, opts)
	if e != nil || len(cross.Contacts) != 4 || len(cross.Material) != 2 {
		t.Fatal("exact 3D wall events and the original bore", cross, e)
	}
	for i, hit := range cross.Contacts {
		if hit.Cap != -1 || hit.StockFace < 0 || hit.Entry != (i%2 == 0) {
			t.Fatal("stock-wall ancestry and crossing orientation", i, hit)
		}
		poly := []*big.Rat{q(-int64(i), 1), q(1, 1)}
		if i == 2 {
			poly = []*big.Rat{q(-25, 8), q(0, 1), q(1, 1)}
		}
		sign, e := hit.Parameter.SignPolynomial(ctx, poly, opts)
		if e != nil || sign.Sign != 0 {
			t.Fatal("wrong rational or algebraic line parameter", i, sign, e)
		}
	}
	oblique := [3]*big.Rat{q(1, 4), q(0, 1), q(1, 16)}
	enter := [3]*big.Rat{q(1, 2), q(7, 4), q(31, 64)}
	cross, e = fiberModel.IntersectRationalLine(ctx, enter, oblique, lineExtent, opts)
	if e != nil || len(cross.Contacts) != 2 || len(cross.Material) != 1 {
		t.Fatal("an oblique line must cross the caps and discard out-of-band walls", cross, e)
	}
	for i, hit := range cross.Contacts {
		if hit.Cap != i || hit.StockFace != -1 || hit.Entry != (i == 0) {
			t.Fatal("cap ancestry and crossing orientation", i, hit)
		}
		want := [2]*big.Rat{q(1, 4), q(1, 1)}[i]
		sign, e := hit.Parameter.SignPolynomial(ctx, []*big.Rat{new(big.Rat).Neg(want), q(1, 1)}, opts)
		if e != nil || sign.Sign != 0 {
			t.Fatal("wrong cap line parameter", i, sign, e)
		}
	}
	if partial, e := fiberModel.IntersectRationalLine(ctx, enter, oblique, lineExtent, geom.ExactCurvePlaneEventOptions{MaxOperations: 1}); !errors.Is(e, geom.ErrConvergence) || len(partial.Contacts)+len(partial.Material) != 0 {
		t.Fatal("exact work exhaustion must publish no line events", partial, e)
	}
	fiber, e := fiberModel.VerticalFiber(ctx, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(7, 4)})
	if e != nil || !fiber.Found || fiber.BoundaryOnly || fiber.Heights[0].Cmp(big.NewRat(1, 2)) != 0 ||
		fiber.Heights[1].Cmp(big.NewRat(35, 64)) != 0 {
		t.Fatal("exact vertical interval between both original caps", fiber, e)
	}
	// At (x,y)=(7/4,177/128), slope dy/dx=7/16 on the original
	// sheared bore. Its tangent remains inside material on both sides.
	anchor := [3]*big.Rat{q(0, 1), q(79, 128), q(1025, 2048)}
	tangent := [3]*big.Rat{q(1, 1), q(7, 16), q(0, 1)}
	window = [2]*big.Rat{q(-1, 1), q(4, 1)}
	if failed, e := fiberModel.IntersectRationalLine(ctx, anchor, tangent, window, opts); !errors.Is(e, geom.ErrConvergence) || len(failed.Contacts) != 0 {
		t.Fatal("legacy crossings-only line should reject a touching contact", failed, e)
	}
	touch, e := fiberModel.IntersectRationalLineWithTouches(ctx, anchor, tangent, window, opts)
	if e != nil || len(touch.Material) != 1 || len(touch.Contacts) != 3 || !touch.Contacts[1].Touch || touch.Contacts[1].Entry || touch.Contacts[1].StockFace < 0 {
		t.Fatal("curve tangent must preserve the one material interval and shared wall owner", touch, e)
	}
	event, e := touch.Contacts[1].Parameter.SignPolynomial(ctx, []*big.Rat{q(-7, 4), q(1, 1)}, opts)
	if e != nil || event.Sign != 0 {
		t.Fatal("wrong repeated exact wall station", event, e)
	}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	for _, side := range []int64{-1, 0, 1} {
		p := [3]*big.Rat{big.NewRat(1, 2), big.NewRat(7, 4),
			new(big.Rat).Add(big.NewRat(35, 64), new(big.Rat).Mul(eps, big.NewRat(side, 1)))}
		got, e := fiberModel.ClassifyRational(ctx, p)
		want := map[int64]parametric.Location{-1: parametric.Inside, 0: parametric.Boundary, 1: parametric.Outside}[side]
		if e != nil || got != want {
			t.Fatal("exact sub-ULP vertical membership", side, got, want, e)
		}
	}
	seam, e := fiberModel.VerticalFiber(ctx, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(11, 8)})
	if e != nil || !seam.Found || !seam.BoundaryOnly || seam.Heights[0].Cmp(seam.Heights[1]) != 0 {
		t.Fatal("closed tip lies on both caps", seam, e)
	}
	void, e := fiberModel.VerticalFiber(ctx, [2]*big.Rat{big.NewRat(3, 2), big.NewRat(7, 4)})
	if e != nil || void.Found {
		t.Fatal("through-void projection has no material fiber", void, e)
	}
	for _, probe := range []struct {
		p    geom.Vec3
		want parametric.Location
	}{
		{geom.Vec3{X: .5, Y: 1.75, Z: .52}, parametric.Inside},
		{geom.Vec3{X: 1.5, Y: 1.75, Z: .52}, parametric.Outside},
		{geom.Vec3{X: .5, Y: 1.4, Z: .48}, parametric.Outside},
		{geom.Vec3{X: .5, Y: 11. / 8, Z: .5}, parametric.Boundary},
		{geom.Vec3{X: 1.5, Y: 11. / 8, Z: .5}, parametric.Outside},
	} {
		got, e := restored.LocateMaterial(ctx, probe.p, SplineShellBooleanOptions{})
		if e != nil || got != probe.want {
			t.Fatal("exact source material and both halfspaces", probe, got, e)
		}
	}
	if len(owners) != attachments {
		t.Fatal("each retained cap side must appear once on a closed stock loop", len(owners), attachments)
	}
	if count != 1 {
		t.Fatal("expected exactly one irrational shared vertex", count)
	}
	cap := RationalGraphShellRegionOptions{Topology: trimvalidate.Options{MaxWork: r.ExactOperations}}
	if bad, e := stock.ClipAlgebraicGraphCapPair(ctx, planes, section, cap); !errors.Is(e, geom.ErrConvergence) || len(bad.Seams)+len(bad.Caps[0].Wires) != 0 {
		t.Fatal("exhausting common pair work must fail atomically", e)
	}
	// The gap over the original through-void must stay outside both caps
	// wherever their planes overlap.
	for _, q := range r.Caps {
		p := [3]*big.Rat{big.NewRat(3, 2), big.NewRat(11, 8), big.NewRat(1, 2)}
		if loc, e := q.LocateRational(ctx, p); e != nil || loc != trimvalidate.Outside {
			t.Fatal("void gap must not become a common seam", loc, e)
		}
	}
}

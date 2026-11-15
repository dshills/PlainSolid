package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestAlgebraicTubeHigherDegreeFaceTrims(t *testing.T) {
	ctx := context.Background()
	for _, axes := range [][2]int{{0, 1}, {0, 2}} {
		t.Run(map[[2]int]string{{0, 1}: "XY", {0, 2}: "XZ"}[axes], func(t *testing.T) {
			var patches [2]intersect.Patch
			var exact [2]*geom.SurfacePatch
			var box geom.SurfacePairParameterBox
			for i, limits := range [2][2]float64{{0, 1}, {-.25, 1.25}} {
				var poles []geom.Vec3
				for _, u := range limits {
					for _, v := range []float64{0, 1} {
						h := float64(i) * (v - .5)
						p := geom.Vec3{X: u, Y: v, Z: h}
						if axes[1] == 2 {
							p.Y, p.Z = p.Z, p.Y
						}
						poles = append(poles, p)
					}
				}
				lo, hi := limits[0], limits[1]
				s, err := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{lo, lo, hi, hi}, []float64{0, 0, 1, 1}, nil)
				if err != nil {
					t.Fatal(err)
				}
				patches[i] = intersect.Patch{Surface: s, Min: geom.Vec2{X: lo}, Max: geom.Vec2{X: hi, Y: 1}}
				exact[i], err = geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				d := geom.ParameterRectangle{UMin: lo, UMax: hi, VMax: 1}
				if i == 0 {
					box.A = d
				} else {
					box.B = d
				}
			}
			tube, err := geom.CertifySurfacePairTube(exact[0], exact[1], geom.SurfacePairTubeOptions{FreeAxis: 0})
			if err != nil || !tube.ProvenBranch {
				t.Fatal("cannot construct fixture's exact pair tube", tube, err)
			}
			seeds := intersect.SplineSeedReport{Attempts: []intersect.SplineSeedCell{{Cell: geom.SurfaceSurfaceCell{DomainA: box.A, DomainB: box.B}, Tube: &tube}}}
			makeWire := func(p [4]geom.Vec2, curve PCurve) []Use {
				out := make([]Use, 4)
				for i := range out {
					out[i] = Use{Edge: 40 + i, UVStart: p[i], UVEnd: p[(i+1)%4]}
				}
				out[0].Curve = curve
				return out
			}
			bounds := makeWire([4]geom.Vec2{{X: -.25}, {X: 1.25}, {X: 1.25, Y: 1}, {X: -.25, Y: 1}}, nil)
			for _, degree := range []int{3, 4} {
				var curve PCurve
				var start, end geom.Vec2
				var rootRecipes [2]geom.ExactRootRecipe
				base := SplineTubeTrimSource{Loop: 0, Use: 0, Edge: 40}
				if degree == 3 {
					c, err := geom.NewBSpline(3, []geom.Vec3{{X: 1, Y: .3125}, {X: .75, Y: .8125}, {X: .25, Y: .8125}, {Y: .3125}}, []float64{2, 2, 2, 2, 6, 6, 6, 6}, nil)
					if err != nil {
						t.Fatal(err)
					}
					curve = SplineUV{Curve: c, Range: UVInterval{2, 6}, Reversed: true}
					start, end = geom.Vec2{Y: .3125}, geom.Vec2{X: 1, Y: .3125}
					base.ParameterStart, base.ParameterEnd = 6, 2
					rootRecipes = [2]geom.ExactRootRecipe{
						{Version: 1, Coefficients: []string{"14", "-8", "1"}, Lower: "5", Upper: "6"},
						{Version: 1, Coefficients: []string{"14", "-8", "1"}, Lower: "2", Upper: "3"},
					}
				} else {
					// U/W=t. V-W/2=-(t-1/4)(t-3/4)(1+t/2+t²/4).
					// These positive homogeneous quartic coefficients preserve
					// both exact crossings and the independent directed weight.
					r := big.NewRat
					h := [][3]*big.Rat{
						{r(0, 1), r(5, 16), r(1, 1)},
						{r(1, 4), r(77, 128), r(9, 8)},
						{r(7, 12), r(307, 384), r(5, 4)},
						{r(1, 1), r(27, 32), r(11, 8)},
						{r(3, 2), r(27, 64), r(3, 2)},
					}
					c, err := NewExactBezierUV(h)
					if err != nil {
						t.Fatal(err)
					}
					curve, start, end = c, c.ends[0], c.ends[1]
					base.ParameterEnd = 1
					rootRecipes = [2]geom.ExactRootRecipe{
						{Version: 1, Coefficients: []string{"-1/4", "1"}, Lower: "1/4", Upper: "1/4"},
						{Version: 1, Coefficients: []string{"-3/4", "1"}, Lower: "3/4", Upper: "3/4"},
					}
				}
				wire := makeWire([4]geom.Vec2{start, end, {X: 1, Y: 1}, {Y: 1}}, curve)
				wire[0].Reversed = true
				base.Reversed = true
				faces := [2]Face{{Surface: patches[0].Surface, Outer: wire}, {Surface: patches[1].Surface, Outer: bounds}}
				o := AlgebraicTrimmedPairSectionOptions{}
				o.Clip.Projection = &axes
				r, err := SectionTrimmedSplinePair(ctx, patches, faces, &seeds, o)
				if err != nil || !r.Complete || len(r.Branches) != 1 {
					t.Fatalf("degree %d original trim section, complete=%v trace=%v exact=%d loop=%d: %v %v", degree, r.Complete, r.Trace.Complete, r.ExactOperations, r.LoopWork, err, r.Trace.Issues)
				}
				check := func(r AlgebraicTrimmedPairSection) {
					t.Helper()
					if len(r.Branches[0].Runs) != 2 {
						t.Fatal("the curved trim must leave two open section runs", r.Branches[0].Runs)
					}
					var contacts []SplineTubeTrimContact
					for _, clip := range r.Branches[0].Clips {
						for _, hit := range clip.FaceTrims[0].Contacts {
							if hit.Source.Use == 0 {
								contacts = append(contacts, hit)
							}
						}
					}
					if len(contacts) != 2 {
						t.Fatal("missing original cubic or quartic coedge contacts", degree, contacts)
					}
					for _, hit := range contacts {
						if hit.Source != base || hit.Multiplicity != 1 || !hit.Transverse || hit.SideSigns[0]*hit.SideSigns[1] != -1 {
							t.Fatal("original directed trim ancestry or crossing signs lost", degree, hit.Source, hit.SideSigns)
						}
					}
					seen := [2]bool{}
					for i, recipe := range rootRecipes {
						root, err := geom.ImportExactRoot(ctx, recipe, geom.ExactRootCodecOptions{})
						if err != nil {
							t.Fatal(err)
						}
						for _, hit := range contacts {
							q, err := root.Compare(ctx, hit.Parameter, geom.ExactCurvePlaneEventOptions{})
							if err != nil {
								t.Fatal(err)
							}
							seen[i] = seen[i] || q.Proven && q.Order == 0
						}
					}
					if seen != ([2]bool{true, true}) {
						t.Fatal("incorrect original algebraic trim parameters", degree, contacts)
					}
					for i := range r.Branches[0].Runs {
						part, err := r.PartitionFaceByTubeRun(ctx, 0, 0, i, AlgebraicTubeFacePartitionOptions{})
						if err != nil || part.SourceSurface != r.runSource().faces[0].Surface || len(part.Cut) != 1 || part.CutClosed || part.Fragments[0].CutReversed == part.Fragments[1].CutReversed {
							t.Fatal("original high-degree coedge could not be partitioned", degree, i, err)
						}
						sourceEdges := 0
						for _, f := range part.Fragments {
							for _, use := range f.Outer {
								if use.Edge == 40 && use.Use == 0 && use.Reversed {
									sourceEdges++
								}
							}
						}
						if sourceEdges != 2 {
							t.Fatal("partition lost portions of the original high-degree edge", degree, i, sourceEdges)
						}
					}
				}
				check(r)
				var data bytes.Buffer
				if err := WriteTrimmedSplinePairSectionJSON(&data, r); err != nil {
					t.Fatal(err)
				}
				back, err := ReadTrimmedSplinePairSectionJSON(ctx, &data, JSONLimits{}, o)
				if err != nil || !back.Complete {
					t.Fatal("higher-degree original trim reload", degree, err)
				}
				check(back)
			}
		})
	}
}

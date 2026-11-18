//go:build research

package trimmed

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ParabolaCirclePocketDiskHistory binds the four transverse original links
// and the separate circular seam between the bounded pocket and its disk.
// The disk vertices and edges start at physical +X and run counterclockwise.
type ParabolaCirclePocketDiskHistory struct {
	ParabolaCircleFourCrossingsHistory
	CrossingCircle int
	PocketCircle   int
	DiskFace       int
	DiskOwner      int
	DiskEdges      [4]int
	DiskVertices   [4]int
	// DiskSignedVolumePiCoefficient is the signed integral of A.Z-B.Z
	// over the original small disk, divided by π.
	DiskSignedVolumePiCoefficient *big.Rat
}

// ParabolaCirclePocketDiskEnvelope is a native original-source Common or
// Fuse with a disk contained in the bounded four-crossing pocket.
type ParabolaCirclePocketDiskEnvelope struct {
	solid     Solid
	history   ParabolaCirclePocketDiskHistory
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

func (r *ParabolaCirclePocketDiskEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *ParabolaCirclePocketDiskEnvelope) History() (h ParabolaCirclePocketDiskHistory) {
	if r == nil {
		return h
	}
	h = r.history
	h.CircleEdges = slices.Clone(h.CircleEdges)
	if h.DiskSignedVolumePiCoefficient != nil {
		h.DiskSignedVolumePiCoefficient = new(big.Rat).Set(h.DiskSignedVolumePiCoefficient)
	}
	return h
}

// CombineParabolaCirclePocketDiskSplineGraphs assembles one exact Common or
// Fuse. The bounded pocket retains its original inner wire and the disk
// has the opposite original roof owner. Every exact native edge and vertex
// is authenticated, including the separate rim of the thin disk region.
func CombineParabolaCirclePocketDiskSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	op parametric.BooleanOp, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCirclePocketDiskEnvelope, error) {
	if op != parametric.Intersection && op != parametric.Union {
		return nil, geom.ErrInvalid
	}
	src := [3]*geom.BSplineSurface{base, a, b}
	p, err := proveParabolaCirclePocketDiskPartition(ctx, src, axis, tolerance, o)
	if err != nil {
		return nil, fmt.Errorf("trimmed: original pocket disk partition: %w", err)
	}
	solid, assembly, err := combineOpenGraphRegions(ctx, base, a, b, p.regions[:], op, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, fmt.Errorf("trimmed: original pocket disk shell: %w", err)
	}
	h, err := bindParabolaCirclePocketDiskEnvelope(ctx, solid, assembly, p, src, op, o.Proof)
	if err != nil {
		return nil, fmt.Errorf("trimmed: original pocket disk ownership: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &ParabolaCirclePocketDiskEnvelope{solid: solid, history: h, sources: src, axis: axis, op: op, tolerance: tolerance}, nil
}

func bindParabolaCirclePocketDiskEnvelope(ctx context.Context, s Solid, assembly SplineGraphBooleanHistory,
	p parabolaCirclePocketDiskPartition, src [3]*geom.BSplineSurface, op parametric.BooleanOp,
	opts geom.GraphPointContactOptions) (h ParabolaCirclePocketDiskHistory, err error) {
	n := len(p.crossing.pieces) + len(h.DiskEdges)
	topo := s.Topology()
	if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 || topo.InnerWires != 1 ||
		topo.Faces != 14 || topo.Edges != n+18 || topo.Vertices != n+7 ||
		len(assembly.ContourEdges) != n || len(assembly.WallFaces) != 6 || assembly.Base != 7 {
		return h, geom.ErrConvergence
	}
	h.ParabolaCircleFourCrossingsHistory, err = bindParabolaCircleFourCrossingEnvelopeRegions(ctx, s, assembly, p.crossing, src, op)
	if err != nil {
		return h, err
	}
	h.CrossingCircle, h.PocketCircle, h.DiskFace = p.proof.CrossingCircle, p.proof.PocketCircle, 6
	h.DiskSignedVolumePiCoefficient = p.proof.DiskSignedVolumePiCoefficient
	if (p.regions[6].sign > 0) != (op == parametric.Union) {
		h.DiskOwner = 1
	}
	if h.DiskOwner == h.RoofOwners[2] {
		return h, geom.ErrConvergence
	}
	disk, pocket := s.faces[h.DiskFace], s.faces[h.RoofFaces[2]]
	if err := bindParabolaCirclesRegionFace(ctx, disk, src[1+h.DiskOwner], false, p.regions[6], p.proof.RegionWitnesses[6]); err != nil {
		return h, err
	}
	edges, err := bindParabolaCircleCrossingEdges(s, assembly.ContourEdges, p.regions[6].loop)
	if err != nil {
		return h, err
	}
	for _, id := range edges {
		if !opposedGraphContourUses(pocket.Holes[0], disk.Outer, id) {
			return h, geom.ErrConvergence
		}
	}
	h.DiskEdges = [4]int(edges)
	h.DiskVertices, err = bindOriginalSeparatedCircleRing(s, edges, p.regions[6].loop, p.proof.PocketExtrema)
	if err != nil {
		return h, err
	}
	if err := materialVertexLinks(ctx, s, true); err != nil {
		return h, err
	}
	return h, validateExactGraphPartitionEdges(ctx, s, opts)
}

const nativeParabolaCirclePocketDiskEnvelope = "trimmed-spline-graph-parabola-circle-pocket-disk-envelope"

type parabolaCirclePocketDiskEnvelopeRecipe struct {
	parabolaCrossingCirclesEnvelopeRecipe
	Arrangement *parabolaCirclePocketDiskEnvelopeLedger `json:"arrangement"`
}

type parabolaCirclePocketDiskEnvelopeLedger struct {
	CrossingCircle                *int   `json:"crossingCircle"`
	PocketCircle                  *int   `json:"pocketCircle"`
	DiskFace                      *int   `json:"diskFace"`
	DiskOwner                     *int   `json:"diskOwner"`
	DiskEdges                     []int  `json:"diskEdges"`
	DiskVertices                  []int  `json:"diskVertices"`
	DiskSignedVolumePiCoefficient string `json:"diskSignedVolumePiCoefficient"`
}

func pocketDiskEnvelopeLedger(h ParabolaCirclePocketDiskHistory) *parabolaCirclePocketDiskEnvelopeLedger {
	return &parabolaCirclePocketDiskEnvelopeLedger{CrossingCircle: &h.CrossingCircle, PocketCircle: &h.PocketCircle,
		DiskFace: &h.DiskFace, DiskOwner: &h.DiskOwner, DiskEdges: h.DiskEdges[:], DiskVertices: h.DiskVertices[:], DiskSignedVolumePiCoefficient: h.DiskSignedVolumePiCoefficient.RatString()}
}

// WriteJSON stores the original supports, four vertex fans and separate
// disk's inner and outer wire incidence.
func (r *ParabolaCirclePocketDiskEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaCirclePocketDiskEnvelopeRecipe{
		parabolaCrossingCirclesEnvelopeRecipe: parabolaCrossingCirclesEnvelopeRecipe{
			Version: 1, Kind: nativeParabolaCirclePocketDiskEnvelope, Sources: src, Axis: &r.axis, Operation: &r.op, Tolerance: r.tolerance,
			Ledger: parabolaCircleFourCrossingsEnvelopeLedgerFor(r.history.ParabolaCircleFourCrossingsHistory),
		},
		Arrangement: pocketDiskEnvelopeLedger(r.history),
	})
}

// ReadParabolaCirclePocketDiskEnvelopeJSON re-proves the original source
// arrangement, hole, disk and four transverse native links.
func ReadParabolaCirclePocketDiskEnvelopeJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCirclePocketDiskEnvelope, error) {
	var recipe parabolaCirclePocketDiskEnvelopeRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCirclePocketDiskEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || recipe.Ledger == nil || recipe.Arrangement == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	r, err := CombineParabolaCirclePocketDiskSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, parabolaCircleFourCrossingsEnvelopeLedgerFor(r.history.ParabolaCircleFourCrossingsHistory)) ||
		!reflect.DeepEqual(recipe.Arrangement, pocketDiskEnvelopeLedger(r.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{r.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}

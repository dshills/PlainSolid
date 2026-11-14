package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/ellipticprism"
)

// PlacedConicCircularPrismMaterial is an explicitly placed filled or annular
// material. Exactly one local topology pointer is nonnil. Original local model
// coefficients and world source vertices/terminals remain separate.
type PlacedConicCircularPrismMaterial struct {
	Filled                  *EllipticalPrismTopology
	Annular                 *EllipticalAnnularPrismTopology
	Placement               geom.Affine
	SourceTransformID       int
	SourceTransformMetadata CylinderDatumMetadata
	SourceVertices          []geom.Vec3
	SourceTerminalPoints    []geom.Vec3
	Solid                   trimmed.Solid
	NativeFaceSourceIDs     []int
	NativeRimEdges          [2][2][4]int
	source                  []byte
}

func ReadPlacedConicCircularPrismMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (PlacedConicCircularPrismMaterial, error) {
	bad := func() (PlacedConicCircularPrismMaterial, error) {
		return PlacedConicCircularPrismMaterial{}, fmt.Errorf("iges: unsupported placed exact ellipse material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return PlacedConicCircularPrismMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 29)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, false, false, true, true, true, true, true)
	if e != nil {
		return PlacedConicCircularPrismMaterial{}, e
	}
	n := len(doc.entities)
	if n != 21 && n != 29 || mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	source := append([]byte(nil), doc.bytes...)
	tr := doc.entities[n-1]
	if tr.kind != 124 || tr.id != 2*n-1 || tr.transform != 0 || len(tr.values) != 13 || compositeSourceNumbers(124, tr.values) != nil {
		return bad()
	}
	k, e := intValue(tr.values[0])
	if e != nil || k != 124 {
		return bad()
	}
	pose, e := decodeMatrix(tr.values)
	if e != nil || !roofRigid(pose) {
		return bad()
	}
	out := PlacedConicCircularPrismMaterial{Placement: pose, SourceTransformID: tr.id, SourceTransformMetadata: CylinderDatumMetadata{tr.label, tr.subscript}, source: source}
	for i, rec := range doc.entities[:n-1] {
		want := 0
		if ellipsePlacedLeaf(rec.kind) {
			want = tr.id
		}
		if rec.transform != want {
			return bad()
		}
		doc.entities[i].transform = 0
		if rec.kind != 502 && rec.kind != 122 {
			continue
		}
		if compositeSourceNumbers(110, rec.values) != nil {
			return bad()
		}
		count := 1
		start := 2
		if rec.kind == 502 {
			count, e = intValue(rec.values[1])
			if e != nil || (count != 2 && count != 4) || len(rec.values) != 2+count*3 {
				return bad()
			}
		} else if len(rec.values) != 5 {
			return bad()
		}
		v := append([]parameter(nil), rec.values...)
		for j := 0; j < count; j++ {
			p := [3]float64{}
			for axis := range p {
				p[axis], e = realValue(v[start+3*j+axis])
				if e != nil {
					return bad()
				}
			}
			world := geom.Vec3{X: p[0], Y: p[1], Z: p[2]}
			if rec.kind == 502 {
				out.SourceVertices = append(out.SourceVertices, world)
			} else {
				out.SourceTerminalPoints = append(out.SourceTerminalPoints, world)
			}
			local, er := roofMapPoint(pose, world, true)
			if er != nil {
				return bad()
			}
			for axis, x := range []float64{local.X, local.Y, local.Z} {
				v[start+3*j+axis].text = number(x)
			}
		}
		doc.entities[i].values = v
	}
	doc.entities = doc.entities[:n-1]
	raw, e := mixedRoofFile(doc)
	if e != nil {
		return bad()
	}
	var d EllipticalAnnularPrismDescriptor
	if n == 21 {
		m, er := ReadConicCircularPrismMaterial(ctx, bytes.NewReader(raw), l, tol)
		if er != nil {
			return bad()
		}
		out.Filled = &m.Topology
		d = EllipticalAnnularPrismDescriptor{OuterCoefficients: m.Topology.Descriptor.Coefficients, LowerZ: m.Topology.Descriptor.LowerZ, UpperZ: m.Topology.Descriptor.UpperZ}
		out.NativeFaceSourceIDs = []int{35, 33, 31, 31, 31, 31}
	} else {
		m, er := ReadConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(raw), l, tol)
		if er != nil {
			return bad()
		}
		out.Annular = &m.Topology
		d = m.Topology.Descriptor
		out.NativeFaceSourceIDs = []int{51, 49, 45, 45, 45, 45, 47, 47, 47, 47}
	}
	origin, e := roofMapPoint(pose, geom.Vec3{Z: d.LowerZ}, false)
	if e != nil {
		return bad()
	}
	frame, e := geom.NewFrame(origin, pose.ApplyVector(geom.Vec3{X: 1}), pose.ApplyVector(geom.Vec3{Z: 1}))
	if e != nil {
		return bad()
	}
	outerProof, e := circularConicProofDescriptor(EllipticalPrismDescriptor{d.OuterCoefficients, d.LowerZ, d.UpperZ})
	if e != nil {
		return bad()
	}
	a, _, e := ellipseConicRadii(EllipseConic{Coefficients: outerProof.Coefficients, Z: d.LowerZ})
	if e != nil {
		return bad()
	}
	height, e := cylinderExactSum(d.UpperZ, -d.LowerZ)
	if e != nil {
		return bad()
	}
	if n == 21 {
		var rim [2][4]int
		out.Solid, rim, e = ellipticprism.CirclePrism(ctx, frame, a, height, tol)
		out.NativeRimEdges[0] = rim
	} else {
		innerProof, er := circularConicProofDescriptor(EllipticalPrismDescriptor{d.InnerCoefficients, d.LowerZ, d.UpperZ})
		if er != nil {
			return bad()
		}
		ia, _, er := ellipseConicRadii(EllipseConic{Coefficients: innerProof.Coefficients, Z: d.LowerZ})
		if er != nil {
			return bad()
		}
		out.Solid, out.NativeRimEdges, e = ellipticprism.CircularAnnulus(ctx, frame, a, ia, height, tol)
	}
	if e != nil {
		return PlacedConicCircularPrismMaterial{}, e
	}
	return out, nil
}

package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// WrappedAnisotropicAssemblySource records one local product shared by 2..16
// flat proper signed-coordinate occurrences. Original supports stay local;
// these occurrence sums are not a geometric material union.
type WrappedAnisotropicAssemblySource struct {
	Assembly      Assembly
	Product       AssemblyProduct
	SourceBodyID  int64
	SourceFaceIDs []int64
	Occurrences   []ExactNAUOOccurrence
}
type WrappedAnisotropicStationAssembly struct {
	WrappedAnisotropicAssemblySource
	Body WrappedAnisotropicStationLoftBody
}
type WrappedAnisotropicStationBoreAssembly struct {
	WrappedAnisotropicAssemblySource
	Body WrappedAnisotropicStationBoreBody
}

func anisotropicAssemblySource(ctx context.Context, r io.Reader, l Limits, nested bool) (WrappedAnisotropicAssemblySource, []byte, error) {
	var zero WrappedAnisotropicAssemblySource
	base, e := readExactFlatSingleProductAssembly(ctx, r, l, nested)
	if e != nil {
		return zero, nil, e
	}
	f := base.Assembly.Source
	closure, e := nauoClosure(f, []int64{base.Product.Representations[0]}, l.MaxDepth)
	if e != nil {
		return zero, nil, e
	}
	view := &File{Header: f.Header, Entities: map[int64]Entity{}}
	for _, id := range f.Order {
		if closure[id] {
			view.Order = append(view.Order, id)
			view.Entities[id] = f.Entities[id]
		}
	}
	raw, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return zero, nil, e
	}
	faces, e := bodyFaceIDs(f, base.SourceBodyID)
	if e != nil {
		return zero, nil, e
	}
	return WrappedAnisotropicAssemblySource{base.Assembly, base.Product, base.SourceBodyID, faces, base.Occurrences}, raw, nil
}

// ReadWrappedAnisotropicStationAssembly proves the original filled local support
// graph once and every independently owned occurrence frame without baking it.
func ReadWrappedAnisotropicStationAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedAnisotropicStationAssembly, error) {
	return readWrappedAnisotropicStationAssembly(ctx, r, l, o, false)
}
func readWrappedAnisotropicStationAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions, nested bool) (WrappedAnisotropicStationAssembly, error) {
	var zero WrappedAnisotropicStationAssembly
	l, e := occurrenceColorSourceLimits(l)
	if e != nil {
		return zero, e
	}
	source, raw, e := anisotropicAssemblySource(ctx, r, l, nested)
	if e != nil {
		return zero, e
	}
	body, e := ReadWrappedAnisotropicStationLoft(bytes.NewReader(raw), l, o)
	if e != nil {
		return zero, e
	}
	if body.LengthUnit != source.Assembly.LengthUnit {
		return zero, geom.ErrDomain
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return WrappedAnisotropicStationAssembly{source, body}, nil
}

// ReadWrappedAnisotropicStationBoreAssembly separately proves both original
// independently scaled wall families and annular caps of the shared local bore.
func ReadWrappedAnisotropicStationBoreAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedAnisotropicStationBoreAssembly, error) {
	return readWrappedAnisotropicStationBoreAssembly(ctx, r, l, o, false)
}
func readWrappedAnisotropicStationBoreAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions, nested bool) (WrappedAnisotropicStationBoreAssembly, error) {
	var zero WrappedAnisotropicStationBoreAssembly
	l, e := occurrenceColorSourceLimits(l)
	if e != nil {
		return zero, e
	}
	source, raw, e := anisotropicAssemblySource(ctx, r, l, nested)
	if e != nil {
		return zero, e
	}
	body, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(raw), l, o)
	if e != nil {
		return zero, e
	}
	if body.LengthUnit != source.Assembly.LengthUnit {
		return zero, geom.ErrDomain
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return WrappedAnisotropicStationBoreAssembly{source, body}, nil
}

type WrappedAnisotropicStationAssemblyPart struct {
	ID, Name string
	Body     *WrappedAnisotropicStationLoftBody
}
type WrappedAnisotropicStationAssemblyModel struct {
	Products    []WrappedAnisotropicStationAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}
type WrappedAnisotropicStationBoreAssemblyPart struct {
	ID, Name string
	Body     *WrappedAnisotropicStationBoreBody
}
type WrappedAnisotropicStationBoreAssemblyModel struct {
	Products    []WrappedAnisotropicStationBoreAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}

func anisotropicAssemblyNames(name string, caps [2]string, walls []WrappedSplineFace) bool {
	if len(name) > 4096 || len(caps[0]) > 4096 || len(caps[1]) > 4096 || len(walls) < 1 || len(walls) > 16 {
		return false
	}
	for _, w := range walls {
		if len(w.Name) > 4096 {
			return false
		}
	}
	return true
}
func WriteWrappedAnisotropicStationAssembly(ctx context.Context, w io.Writer, m WrappedAnisotropicStationAssemblyModel, l Limits, o WrappedSurfaceOptions) error {
	return writeWrappedAnisotropicStationAssembly(ctx, w, m, l, o, false)
}
func writeWrappedAnisotropicStationAssembly(ctx context.Context, w io.Writer, m WrappedAnisotropicStationAssemblyModel, l Limits, o WrappedSurfaceOptions, nested bool) error {
	products := 2
	if nested {
		products = 3
	}
	if len(m.Products) != products {
		return geom.ErrInvalid
	}
	graph := AssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	counts := make([]int, products)
	for i, p := range m.Products {
		graph.Products = append(graph.Products, AssemblyPart{ID: p.ID, Name: p.Name})
		if p.Body != nil {
			if p.Body.LengthUnit != m.LengthUnit || !anisotropicAssemblyNames(p.Body.Name, p.Body.CapNames, p.Body.Walls) {
				return geom.ErrInvalid
			}
			counts[i] = 1
		}
	}
	return writeAnisotropicAssembly(ctx, w, graph, counts, func(i int) (*File, []int64, error) {
		var b bytes.Buffer
		if e := WriteWrappedAnisotropicStationLoft(&b, *m.Products[i].Body); e != nil {
			return nil, nil, e
		}
		return stagedAssemblyBodies(&b)
	}, l, o, false, nested)
}
func WriteWrappedAnisotropicStationBoreAssembly(ctx context.Context, w io.Writer, m WrappedAnisotropicStationBoreAssemblyModel, l Limits, o WrappedSurfaceOptions) error {
	return writeWrappedAnisotropicStationBoreAssembly(ctx, w, m, l, o, false)
}
func writeWrappedAnisotropicStationBoreAssembly(ctx context.Context, w io.Writer, m WrappedAnisotropicStationBoreAssemblyModel, l Limits, o WrappedSurfaceOptions, nested bool) error {
	products := 2
	if nested {
		products = 3
	}
	if len(m.Products) != products {
		return geom.ErrInvalid
	}
	graph := AssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	counts := make([]int, products)
	for i, p := range m.Products {
		graph.Products = append(graph.Products, AssemblyPart{ID: p.ID, Name: p.Name})
		if p.Body != nil {
			if p.Body.LengthUnit != m.LengthUnit || !anisotropicAssemblyNames(p.Body.Name, p.Body.CapNames, p.Body.Outer.Walls) || !anisotropicAssemblyNames(p.Body.Name, p.Body.CapNames, p.Body.Inner.Walls) {
				return geom.ErrInvalid
			}
			counts[i] = 1
		}
	}
	return writeAnisotropicAssembly(ctx, w, graph, counts, func(i int) (*File, []int64, error) {
		var b bytes.Buffer
		if e := WriteWrappedAnisotropicStationBore(&b, *m.Products[i].Body); e != nil {
			return nil, nil, e
		}
		return stagedAssemblyBodies(&b)
	}, l, o, true, nested)
}
func writeAnisotropicAssembly(ctx context.Context, w io.Writer, m AssemblyModel, counts []int, stage func(int) (*File, []int64, error), l Limits, o WrappedSurfaceOptions, bore bool, nested bool) error {
	products, minUses, maxUses := 2, 2, 16
	if nested {
		products, minUses, maxUses = 3, 3, 17
	}
	total := 0
	for _, n := range counts {
		total += n
	}
	if ctx == nil || w == nil || len(counts) != products || total != 1 || len(m.Occurrences) < minUses || len(m.Occurrences) > maxUses {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	for _, p := range m.Products {
		if len(p.ID) > 4096 || len(p.Name) > 4096 {
			return geom.ErrInvalid
		}
	}
	for _, u := range m.Occurrences {
		if len(u.ID) > 4096 || len(u.Name) > 4096 || len(u.Parent) > 4096 || len(u.Child) > 4096 || !planarSplineRigid(u.Transform) {
			return geom.ErrInvalid
		}
	}
	var staged bytes.Buffer
	if e := writeAssemblyGeometry(&staged, m, counts, stage); e != nil {
		return e
	}
	var e error
	if bore {
		_, e = readWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(staged.Bytes()), l, o, nested)
	} else {
		_, e = readWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(staged.Bytes()), l, o, nested)
	}
	if e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		e = io.ErrShortWrite
	}
	return e
}
func WriteWrappedAnisotropicStationAssemblySource(ctx context.Context, w io.Writer, f *File, l Limits, o WrappedSurfaceOptions) error {
	return writeAnisotropicAssemblySource(ctx, w, f, l, o, false)
}
func WriteWrappedAnisotropicStationBoreAssemblySource(ctx context.Context, w io.Writer, f *File, l Limits, o WrappedSurfaceOptions) error {
	return writeAnisotropicAssemblySource(ctx, w, f, l, o, true)
}
func writeAnisotropicAssemblySource(ctx context.Context, w io.Writer, f *File, l Limits, o WrappedSurfaceOptions, bore bool) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(f, 4096, 16) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if bore {
		_, e = ReadWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), l, o)
	} else {
		_, e = ReadWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), l, o)
	}
	if e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

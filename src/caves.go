package main

import (
	"fmt"
	"math"
)

// ---------- colisiones (.gfbcol) ----------

type colSub struct {
	mat uint32
	idx []uint16
}
type colMesh struct {
	subs  []colSub
	flags []map[int]uint32
	verts []float32
}
type colChunk struct {
	f0   uint32
	bbox []float32
}
type colFile struct {
	mats   []string
	chunks []colChunk
	meshes []colMesh
	r2bbox []float32
	r2f1   uint32
}

func scalarFields(fb FB, t int) map[int]uint32 {
	m := map[int]uint32{}
	for i := 0; i < fb.NumFields(t); i++ {
		if p := fb.FieldPos(t, i); p >= 0 {
			m[i] = le.Uint32(fb.B[p:])
		}
	}
	return m
}

func parseCol(d []byte) *colFile {
	fb := FB{d}
	r := fb.Root()
	c := &colFile{}
	for _, m := range fb.Vector(r, 0) {
		c.mats = append(c.mats, fb.Str(m, 0))
	}
	for _, ch := range fb.Vector(r, 1) {
		c.chunks = append(c.chunks, colChunk{fb.U32(ch, 0), fb.Floats(fb.FieldPos(ch, 1), 6)})
	}
	for _, m := range fb.Vector(r, 2) {
		var cm colMesh
		for _, s := range fb.Vector(m, 0) {
			p, n := fb.VecRaw(s, 1)
			idx := make([]uint16, n)
			for k := range idx {
				idx[k] = le.Uint16(fb.B[p+2*k:])
			}
			cm.subs = append(cm.subs, colSub{fb.U32(s, 0), idx})
		}
		for _, g := range fb.Vector(m, 1) {
			cm.flags = append(cm.flags, scalarFields(fb, g))
		}
		p, n := fb.VecRaw(m, 2)
		cm.verts = fb.Floats(p, n/4)
		c.meshes = append(c.meshes, cm)
	}
	t := fb.Table(r, 3)
	c.r2bbox = fb.Floats(fb.FieldPos(t, 0), 6)
	c.r2f1 = fb.U32(t, 1)
	return c
}

func buildCol(c *colFile) []byte {
	mats := &VecT{}
	for _, m := range c.mats {
		mats.Elems = append(mats.Elems, NewTbl().Ref(0, Str(m)))
	}
	chunks := &VecT{}
	for _, ch := range c.chunks {
		t := NewTbl().Floats(1, ch.bbox...)
		if ch.f0 != 0 {
			t.U32(0, ch.f0)
		}
		chunks.Elems = append(chunks.Elems, t)
	}
	meshes := &VecT{}
	for _, m := range c.meshes {
		subs := &VecT{}
		for _, s := range m.subs {
			t := NewTbl().Ref(1, VecU16(s.idx))
			if s.mat != 0 {
				t.U32(0, s.mat)
			}
			subs.Elems = append(subs.Elems, t)
		}
		fl := &VecT{}
		for _, g := range m.flags {
			t := NewTbl()
			for i, v := range g {
				t.U32(i, v)
			}
			fl.Elems = append(fl.Elems, t)
		}
		vb := make([]byte, 4*len(m.verts))
		for i, v := range m.verts {
			le.PutUint32(vb[4*i:], math.Float32bits(v))
		}
		meshes.Elems = append(meshes.Elems, NewTbl().Ref(0, subs).Ref(1, fl).Ref(2, VecBytes(vb)))
	}
	r2 := NewTbl().Floats(0, c.r2bbox...)
	if c.r2f1 != 0 {
		r2.U32(1, c.r2f1)
	}
	return BuildFB(NewTbl().Ref(0, mats).Ref(1, chunks).Ref(2, meshes).Ref(3, r2))
}

// ---------- suelo (para colocar matojos a la altura correcta) ----------

type tri struct {
	p   [3][3]float64
	mat string
}
type floorIdx struct {
	cell float64
	grid map[[2]int][]tri
}

func newFloor(cols []*colFile) *floorIdx {
	f := &floorIdx{cell: 500, grid: map[[2]int][]tri{}}
	for _, c := range cols {
		for _, m := range c.meshes {
			for _, s := range m.subs {
				for t := 0; t+2 < len(s.idx); t += 3 {
					var tr tri
					tr.mat = c.mats[s.mat]
					minx, maxx, minz, maxz := 1e18, -1e18, 1e18, -1e18
					for k := 0; k < 3; k++ {
						j := int(s.idx[t+k])
						tr.p[k] = [3]float64{float64(m.verts[3*j]), float64(m.verts[3*j+1]), float64(m.verts[3*j+2])}
						minx, maxx = math.Min(minx, tr.p[k][0]), math.Max(maxx, tr.p[k][0])
						minz, maxz = math.Min(minz, tr.p[k][2]), math.Max(maxz, tr.p[k][2])
					}
					for gx := int(math.Floor(minx / f.cell)); gx <= int(math.Floor(maxx/f.cell)); gx++ {
						for gz := int(math.Floor(minz / f.cell)); gz <= int(math.Floor(maxz/f.cell)); gz++ {
							f.grid[[2]int{gx, gz}] = append(f.grid[[2]int{gx, gz}], tr)
						}
					}
				}
			}
		}
	}
	return f
}

type hit struct {
	y   float64
	mat string
}

func (f *floorIdx) query(x, z, ynear, dy float64) []hit {
	var out []hit
	for _, t := range f.grid[[2]int{int(math.Floor(x / f.cell)), int(math.Floor(z / f.cell))}] {
		x1, y1, z1 := t.p[0][0], t.p[0][1], t.p[0][2]
		x2, y2, z2 := t.p[1][0], t.p[1][1], t.p[1][2]
		x3, y3, z3 := t.p[2][0], t.p[2][1], t.p[2][2]
		d := (z2-z3)*(x1-x3) + (x3-x2)*(z1-z3)
		if math.Abs(d) < 1e-9 {
			continue
		}
		a := ((z2-z3)*(x-x3) + (x3-x2)*(z-z3)) / d
		b := ((z3-z1)*(x-x3) + (x1-x3)*(z-z3)) / d
		cc := 1 - a - b
		if a >= -1e-6 && b >= -1e-6 && cc >= -1e-6 {
			y := a*y1 + b*y2 + cc*y3
			if math.Abs(y-ynear) <= dy {
				out = append(out, hit{y, t.mat})
			}
		}
	}
	return out
}

// ---------- recorte de triangulos ----------

type v3 = [3]float64

func splitPoly(poly []v3, axis int, val float64, keepLess bool) []v3 {
	var out []v3
	n := len(poly)
	for i := 0; i < n; i++ {
		a, b := poly[i], poly[(i+1)%n]
		ina := (keepLess && a[axis] <= val) || (!keepLess && a[axis] >= val)
		inb := (keepLess && b[axis] <= val) || (!keepLess && b[axis] >= val)
		if ina {
			out = append(out, a)
		}
		if ina != inb {
			t := (val - a[axis]) / (b[axis] - a[axis])
			out = append(out, v3{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t})
		}
	}
	return out
}

func cutTriangle(t [3]v3, x0, z0, x1, z1 float64) (inside, outside [][]v3) {
	cur := [][]v3{{t[0], t[1], t[2]}}
	cuts := []struct {
		axis int
		val  float64
		less bool
	}{{0, x0, false}, {0, x1, true}, {2, z0, false}, {2, z1, true}}
	for _, cu := range cuts {
		var nxt [][]v3
		for _, poly := range cur {
			a := splitPoly(poly, cu.axis, cu.val, cu.less)
			b := splitPoly(poly, cu.axis, cu.val, !cu.less)
			if len(b) >= 3 {
				outside = append(outside, b)
			}
			if len(a) >= 3 {
				nxt = append(nxt, a)
			}
		}
		cur = nxt
	}
	return cur, outside
}

func area2(a, b, cc v3) float64 {
	return math.Abs((b[0]-a[0])*(cc[2]-a[2]) - (cc[0]-a[0])*(b[2]-a[2]))
}

type square struct{ x0, z0, x1, z1, y float64 }

var skipMats = map[string]bool{"col_kawa": true, "col_umi": true, "col_ike": true, "col_asase": true, "col_mizutamari": true,
	"col_kusa": true, "col_yukikusa": true, "col_karekusa": true}

func applyPatches(c *colFile, sqs []square, target string) int {
	tk := -1
	for i, m := range c.mats {
		if m == target {
			tk = i
		}
	}
	if tk < 0 {
		c.mats = append(c.mats, target)
		tk = len(c.mats) - 1
	}
	changed := 0
	for mi := range c.meshes {
		m := &c.meshes[mi]
		var newIdx []uint16
		addv := func(p v3) uint16 {
			m.verts = append(m.verts, float32(p[0]), float32(p[1]), float32(p[2]))
			return uint16(len(m.verts)/3 - 1)
		}
		for si := range m.subs {
			s := &m.subs[si]
			if skipMats[c.mats[s.mat]] {
				continue
			}
			var keep []uint16
			for t := 0; t+2 < len(s.idx); t += 3 {
				ids := s.idx[t : t+3]
				var tr [3]v3
				minx, maxx, minz, maxz, miny, maxy := 1e18, -1e18, 1e18, -1e18, 1e18, -1e18
				for k := 0; k < 3; k++ {
					j := int(ids[k])
					tr[k] = v3{float64(m.verts[3*j]), float64(m.verts[3*j+1]), float64(m.verts[3*j+2])}
					minx, maxx = math.Min(minx, tr[k][0]), math.Max(maxx, tr[k][0])
					minz, maxz = math.Min(minz, tr[k][2]), math.Max(maxz, tr[k][2])
					miny, maxy = math.Min(miny, tr[k][1]), math.Max(maxy, tr[k][1])
				}
				var hs *square
				for qi := range sqs {
					q := &sqs[qi]
					if maxx < q.x0 || minx > q.x1 || maxz < q.z0 || minz > q.z1 || miny > q.y+300 || maxy < q.y-300 {
						continue
					}
					hs = q
					break
				}
				if hs == nil {
					keep = append(keep, ids...)
					continue
				}
				ins, outs := cutTriangle(tr, hs.x0, hs.z0, hs.x1, hs.z1)
				if len(ins) == 0 {
					keep = append(keep, ids...)
					continue
				}
				changed++
				if len(outs) == 0 {
					newIdx = append(newIdx, ids...)
					continue
				}
				for _, poly := range outs {
					for i := 1; i+1 < len(poly); i++ {
						if area2(poly[0], poly[i], poly[i+1]) < 1e-3 {
							continue
						}
						keep = append(keep, addv(poly[0]), addv(poly[i]), addv(poly[i+1]))
					}
				}
				for _, poly := range ins {
					for i := 1; i+1 < len(poly); i++ {
						if area2(poly[0], poly[i], poly[i+1]) < 1e-3 {
							continue
						}
						newIdx = append(newIdx, addv(poly[0]), addv(poly[i]), addv(poly[i+1]))
					}
				}
			}
			s.idx = keep
		}
		if len(newIdx) > 0 {
			found := false
			for si := range m.subs {
				if m.subs[si].mat == uint32(tk) {
					m.subs[si].idx = append(m.subs[si].idx, newIdx...)
					found = true
				}
			}
			if !found {
				m.subs = append(m.subs, colSub{uint32(tk), newIdx})
			}
		}
		var ns []colSub
		for _, s := range m.subs {
			if len(s.idx) > 0 {
				ns = append(ns, s)
			}
		}
		m.subs = ns
		if len(m.verts)/3 >= 65536 {
			fatal("demasiados vertices en una colision")
		}
	}
	return changed
}

// ---------- matojos visibles (.gfbmdli) ----------

type grassModel struct {
	l, s     string
	lbl, lbs []float32
}

var grassModels = map[string]grassModel{
	"01": {"instance_obj_encountweed01/instance_obj_encountweed01_large01", "instance_obj_encountweed01/instance_obj_encountweed01_small01",
		[]float32{-80.191, -5.0, -80.226, 83.507, 77.783, 80.357}, []float32{-57.611, -2.0, -60.812, 57.611, 56.45, 57.92}},
	"04": {"instance_obj_encountweed04/instance_obj_encountweed04_large01", "instance_obj_encountweed04/instance_obj_encountweed04_small01",
		[]float32{-110.0, -15.984, -136.093, 157.146, 93.339, 136.093}, []float32{-81.133, -10.0, -59.698, 81.133, 70.0, 69.504}},
	"11": {"instance_obj_encountweed11/instance_obj_encountweed11_large01", "instance_obj_encountweed11/instance_obj_encountweed11_small01",
		[]float32{-110.0, -15.984, -136.093, 157.146, 93.339, 136.093}, []float32{-81.133, -10.0, -59.698, 81.133, 70.0, 69.504}},
}

type grassPatch struct {
	m     grassModel
	instL []v3
	instS []v3
}

func bboxOf(pts []v3, lb []float32) []float32 {
	b := []float32{1e30, 1e30, 1e30, -1e30, -1e30, -1e30}
	for _, p := range pts {
		for k := 0; k < 3; k++ {
			b[k] = float32(math.Min(float64(b[k]), p[k]+float64(lb[k])))
			b[3+k] = float32(math.Max(float64(b[3+k]), p[k]+float64(lb[3+k])))
		}
	}
	return b
}
func mergeBB(a, b []float32) []float32 {
	if a == nil {
		return b
	}
	r := make([]float32, 6)
	for k := 0; k < 3; k++ {
		r[k] = float32(math.Min(float64(a[k]), float64(b[k])))
		r[3+k] = float32(math.Max(float64(a[3+k]), float64(b[3+k])))
	}
	return r
}

func (c *Ctx) buildGfbmdli(area string, fileid int, patches []grassPatch) ([]byte, []string, []float32) {
	meshes := &VecT{}
	var names []string
	var total []float32
	for pi, p := range patches {
		for _, kind := range []string{"l", "s"} {
			pts, model, lb := p.instL, p.m.l, p.m.lbl
			if kind == "s" {
				pts, model, lb = p.instS, p.m.s, p.m.lbs
			}
			if len(pts) == 0 {
				continue
			}
			wb := bboxOf(pts, lb)
			total = mergeBB(total, wb)
			name := fmt.Sprintf("%s_coord_encountweed%03d_%02d_%s_ReproMesh", area, fileid, pi+1, kind)
			names = append(names, name)
			insts := &VecT{}
			for _, q := range pts {
				insts.Elems = append(insts.Elems, NewTbl().Floats(2, float32(q[0]), float32(q[1]), float32(q[2])).
					Floats(1, 0, float32(c.rng.Float64()*2*math.Pi), 0).Floats(0, 1, 1, 1))
			}
			path := fmt.Sprintf("|%s_coord|instance_obj|%s", area, colonize(model))
			grp := NewTbl().Ref(0, Str(path)).Ref(1, Str(model)).Floats(2, lb...).Floats(3, wb...).Ref(4, insts)
			meshes.Elems = append(meshes.Elems, NewTbl().Ref(0, Str(name)).Floats(1, wb...).Ref(2, &VecT{Elems: []interface{}{grp}}))
		}
	}
	root := NewTbl().U32(0, 0x18072019).Floats(1, total...).Ref(2, meshes)
	return BuildFB(root), names, total
}

func colonize(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] == '/' {
			b[i] = ':'
		}
	}
	return string(b)
}

// anade una entrada de instancias al descriptor de terreno sin tocar las existentes
func addTerrainEntry(old []byte, gpath string, names []string, bbox []float32) []byte {
	u32 := func(p int) int { return int(le.Uint32(old[p:])) }
	root := u32(0)
	vt := root - int(int32(le.Uint32(old[root:])))
	f0 := int(le.Uint16(old[vt+4:]))
	vp := root + f0
	vec := vp + u32(vp)
	n := u32(vec)
	elems := make([]int, n)
	for k := 0; k < n; k++ {
		elems[k] = vec + 4 + 4*k + u32(vec+4+4*k)
	}
	hdr := 4 + 8 + 8 + 4 + 4*(n+1)
	for (hdr-4)%16 != 0 {
		hdr++
	}
	delta := hdr - 4
	out := make([]byte, hdr)
	vtPos, tblPos, vecPos := 4, 12, 20
	le.PutUint32(out[0:], uint32(tblPos))
	le.PutUint16(out[vtPos:], 6)
	le.PutUint16(out[vtPos+2:], 8)
	le.PutUint16(out[vtPos+4:], 4)
	le.PutUint32(out[tblPos:], uint32(int32(tblPos-vtPos)))
	le.PutUint32(out[tblPos+4:], uint32(vecPos-(tblPos+4)))
	le.PutUint32(out[vecPos:], uint32(n+1))
	out = append(out, old[4:]...)
	al := func(a int) {
		for len(out)%a != 0 {
			out = append(out, 0)
		}
	}
	al(16)
	wvt := len(out)
	out = append(out, 6, 0, 8, 0, 4, 0)
	al(4)
	wt := len(out)
	out = append(out, make([]byte, 8)...)
	le.PutUint32(out[wt:], uint32(int32(wt-wvt)))
	ivt := len(out)
	for _, v := range []uint16{22, 60, 4, 8, 12, 16, 20, 24, 28, 52, 56} {
		out = append(out, byte(v), byte(v>>8))
	}
	al(4)
	it := len(out)
	out = append(out, make([]byte, 60)...)
	le.PutUint32(out[it:], uint32(int32(it-ivt)))
	le.PutUint32(out[wt+4:], uint32(it-(wt+4)))
	for k, f := range bbox {
		le.PutUint32(out[it+28+4*k:], math.Float32bits(f))
	}
	putStr := func(s string) int {
		al(4)
		p := len(out)
		out = append(out, 0, 0, 0, 0)
		le.PutUint32(out[p:], uint32(len(s)))
		out = append(out, []byte(s)...)
		out = append(out, 0)
		return p
	}
	strs := map[int]string{0: "", 1: gpath, 2: "", 3: "", 4: "", 5: "", 7: "bin/field/model/instance_obj/"}
	slot := map[int]int{0: 4, 1: 8, 2: 12, 3: 16, 4: 20, 5: 24, 7: 52}
	for _, k := range []int{0, 1, 2, 3, 4, 5, 7} {
		p := putStr(strs[k])
		fp := it + slot[k]
		le.PutUint32(out[fp:], uint32(p-fp))
	}
	al(4)
	vp2 := len(out)
	out = append(out, make([]byte, 4+4*len(names))...)
	le.PutUint32(out[vp2:], uint32(len(names)))
	for i, s := range names {
		p := putStr(s)
		fp := vp2 + 4 + 4*i
		le.PutUint32(out[fp:], uint32(p-fp))
	}
	fp := it + 56
	le.PutUint32(out[fp:], uint32(vp2-fp))
	for k, e := range elems {
		fp := vecPos + 4 + 4*k
		le.PutUint32(out[fp:], uint32(e+delta-fp))
	}
	fp = vecPos + 4 + 4*n
	le.PutUint32(out[fp:], uint32(wt-fp))
	al(16)
	return out
}

// ---------- aplicar parches de hierba a cuevas y zonas sin hierba ----------

type patchZone struct{ zone, model, mat string }

func (c *Ctx) caveGrass(pl *GFPack) {
	areas := []string{"a_d0201", "a_d0302", "a_wr0201", "a_wr0301"}
	zones := map[string][]patchZone{
		"a_d0201": {{"Galar Mine", "01", "col_kusa"}},
		"a_d0302": {{"Galar Mine No. 2", "01", "col_kusa"}},
	}
	for _, z := range dlcCaves {
		pz := patchZone{z.name, "04", "col_kusa"}
		if z.name == "Path to the Peak" {
			pz = patchZone{z.name, "11", "col_yukikusa"}
		}
		zones[z.area] = append(zones[z.area], pz)
	}
	patchSize := map[string]float64{}
	if cityTest() {
		for _, cd := range gymCities {
			found := false
			for _, a := range areas {
				if a == cd.area {
					found = true
				}
			}
			if !found {
				areas = append(areas, cd.area)
			}
			zones[cd.area] = append(zones[cd.area], patchZone{cd.name, "01", "col_kusa"})
		}
	}
	tp, err := ReadGFPack(c.read("bin/archive/field/resident/terrain.gfpak"))
	if err != nil {
		fatal(err.Error())
	}
	total := 0
	for _, ar := range areas {
		ap, err := ReadGFPack(c.read("bin/archive/field/area/other/" + ar + ".gfpak"))
		if err != nil {
			fatal(err.Error())
		}
		var colIdx []int
		cols := map[int]*colFile{}
		var colList []*colFile
		for i, f := range ap.Files {
			if containsBytes(f, "col_") && isCol(f) {
				colIdx = append(colIdx, i)
				cols[i] = parseCol(f)
				colList = append(colList, cols[i])
			}
		}
		fl := newFloor(colList)
		if cityTest() {
			for _, cd := range gymCities {
				if cd.area != ar {
					continue
				}
				site, size, ok := c.citySite(pl, ar, cd.zone, fl)
				if cd.wx != 0 || cd.wz != 0 {
					site, ok = manualSite(fl, cd.wx, cd.wz, cd.half)
					size = cd.half * 2
				}
				if !ok {
					c.logf("AVISO: no encontre sitio para la hierba de %s", cd.name)
					continue
				}
				c.addSpawners(pl, ar, cd.zone, []v3{site})
				patchSize[cd.name] = size
				c.logf("%s: hierba en (%.0f, %.0f, %.0f), %.0fx%.0f", cd.name, site[0], site[1], site[2], size, size)
			}
		}
		fb := FB{pl.Get(ar + ".bin")}
		spawn := map[string][]v3{}
		for _, z := range fb.Vector(fb.Root(), 0) {
			nm := zoneNameOf[fb.U64(fb.Table(z, 0), 1)]
			for _, h := range fb.Vector(z, 20) {
				idt := fb.Table(fb.Table(h, 0), 0)
				spawn[nm] = append(spawn[nm], v3{float64(fb.F32(idt, 0)), float64(fb.F32(idt, 1)), float64(fb.F32(idt, 2))})
			}
		}
		var patches []grassPatch
		sqByMat := map[string][]square{}
		for _, pz := range zones[ar] {
			PATCH := 500.0
			if s, ok := patchSize[pz.zone]; ok {
				PATCH = s
			}
			for _, cl := range clusters(spawn[pz.zone]) {
				x0, z0, x1, z1 := cl[0]-PATCH/2, cl[2]-PATCH/2, cl[0]+PATCH/2, cl[2]+PATCH/2
				if len(fl.query(cl[0], cl[2], cl[1], 300)) == 0 {
					continue
				}
				sqByMat[pz.mat] = append(sqByMat[pz.mat], square{x0, z0, x1, z1, cl[1]})
				gp := grassPatch{m: grassModels[pz.model]}
				for _, kind := range []struct {
					sp float64
					l  bool
				}{{45, true}, {60, false}} {
					n := int(PATCH / kind.sp)
					for i := 0; i < n; i++ {
						for j := 0; j < n; j++ {
							x := x0 + (float64(i)+0.5)*kind.sp + (c.rng.Float64()*0.6-0.3)*kind.sp
							zz := z0 + (float64(j)+0.5)*kind.sp + (c.rng.Float64()*0.6-0.3)*kind.sp
							best, ok := 0.0, false
							for _, h := range fl.query(x, zz, cl[1], 300) {
								if skipMats[h.mat] {
									continue
								}
								if !ok || math.Abs(h.y-cl[1]) < math.Abs(best-cl[1]) {
									best, ok = h.y, true
								}
							}
							if ok {
								if kind.l {
									gp.instL = append(gp.instL, v3{x, best, zz})
								} else {
									gp.instS = append(gp.instS, v3{x, best, zz})
								}
							}
						}
					}
				}
				if len(gp.instL) > 0 {
					patches = append(patches, gp)
				}
			}
		}
		ch := 0
		for _, i := range colIdx {
			n := 0
			for mat, sqs := range sqByMat {
				n += applyPatches(cols[i], sqs, mat)
			}
			if n > 0 {
				ap.Files[i] = buildCol(cols[i])
				ch += n
			}
		}
		c.put("bin/archive/field/area/other/"+ar+".gfpak", ap.Write())
		gpath := fmt.Sprintf("bin/field/model/%s/%s_coord_encountweed900.gfbmdli", ar, ar)
		data, names, bb := c.buildGfbmdli(ar, 900, patches)
		c.put(gpath, data)
		idx := -1
		for i, f := range tp.Files {
			if containsBytes(f, "bin/field/model/"+ar+"/"+ar+"_") && containsBytes(f, ".gfbcol") {
				if idx >= 0 {
					fatal("descriptor de terreno ambiguo para " + ar)
				}
				idx = i
			}
		}
		if idx < 0 {
			fatal("no encuentro el descriptor de terreno de " + ar)
		}
		tp.Files[idx] = addTerrainEntry(tp.Files[idx], gpath, names, bb)
		total += len(patches)
		c.logf("%s: %d parches de hierba (%d triangulos de suelo recortados)", ar, len(patches), ch)
	}
	c.put("bin/archive/field/resident/terrain.gfpak", tp.Write())
	_ = total
}

func clusters(pts []v3) []v3 {
	type cl struct {
		c   v3
		pts []v3
	}
	var cs []*cl
	for _, p := range pts {
		placed := false
		for _, c := range cs {
			if math.Hypot(p[0]-c.c[0], p[2]-c.c[2]) < 450 && math.Abs(p[1]-c.c[1]) < 200 {
				c.pts = append(c.pts, p)
				var s v3
				for _, q := range c.pts {
					s[0] += q[0]
					s[1] += q[1]
					s[2] += q[2]
				}
				n := float64(len(c.pts))
				c.c = v3{s[0] / n, s[1] / n, s[2] / n}
				placed = true
				break
			}
		}
		if !placed {
			cs = append(cs, &cl{p, []v3{p}})
		}
	}
	var r []v3
	for _, c := range cs {
		r = append(r, c.c)
	}
	return r
}

func containsBytes(b []byte, s string) bool {
	n := len(s)
	for i := 0; i+n <= len(b); i++ {
		if string(b[i:i+n]) == s {
			return true
		}
	}
	return false
}

// es un archivo de colision con materiales? (raiz con 4 campos y el primero lista de materiales col_*)
func isCol(d []byte) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	fb := FB{d}
	r := fb.Root()
	if fb.NumFields(r) != 4 {
		return false
	}
	ms := fb.Vector(r, 0)
	if len(ms) == 0 {
		return false
	}
	s := fb.Str(ms[0], 0)
	return len(s) > 4 && s[:4] == "col_"
}

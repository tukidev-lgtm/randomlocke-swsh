package main

import (
	"math"
	"os"
)

// Prueba: hierba alta en las ciudades con gimnasio.
type cityDef struct {
	area string
	zone uint64
	ref  string // ruta cercana de la que se toman los niveles
	name string
	wx   float64 // sitio elegido a mano (0 = automatico)
	wz   float64
	half float64
}

var gymCities = []cityDef{
	{"a_0201", 0xE0D6E5E78C91F4A7, "Route 3", "Motostoke", 0, 0, 0},
	{"a_0301", 0xF12207C040C1CE88, "Route 4", "Turffield", 0, 0, 0},
	{"a_0401", 0x1C7150C0594994E5, "Route 5", "Hulbury", 0, 0, 0},
	{"a_0501", 0xFC1954E79C5EAFE2, "Hammerlocke Hills", "Hammerlocke", 0, 0, 0},
	{"a_0601", 0x23D489C05D1C60CA, "Route 6", "Stow-on-Side", 0, 0, 0},
	{"a_0701", 0x0AB1FAC04F1DD58F, "Glimwood Tangle", "Ballonlea", 2650, 8800, 175},
	{"a_0901", 0x13C833C0546235F4, "Route 8", "Circhester", 5000, 8900, 250},
	{"a_1001", 0x3F177CC06CE9FC51, "Route 9 (in Outer Spikemuth)", "Spikemuth", 6000, 2700, 175},
}

func cityTest() bool { return os.Getenv("RLK_CIUDADES") != "0" }

// tablas ocultas propias para cada ciudad (especies nuevas, niveles de la ruta cercana)
func (c *Ctx) cityTables(H *encArchive, pc *poolCache) {
	Ht := map[uint64]*encTable{}
	for _, t := range H.tables {
		Ht[t.zone] = t
	}
	for _, cd := range gymCities {
		ref := Ht[zoneHash(cd.ref, false)]
		var subs []encSub
		for i := 0; i < 11; i++ {
			s := encSub{slots: make([]encSlot, 10)}
			if i < 9 {
				s.min, s.max = ref.subs[0].min, ref.subs[0].max
				probs := []uint8{30, 25, 20, 15, 10}
				for k, p := range probs {
					s.slots[k] = encSlot{p, int32(pc.pick(int(s.max), true)), 0}
				}
			}
			subs = append(subs, s)
		}
		if t, ok := Ht[cd.zone]; ok {
			t.subs = subs
		} else {
			H.tables = append(H.tables, &encTable{zone: cd.zone, subs: subs})
		}
		zoneNameOf[cd.zone] = cd.name
	}
}

// posicion de aterrizaje (vuelo) de cada ciudad: zona abierta junto al Centro Pokemon
func (c *Ctx) cityAnchors(pl *GFPack, area string, zone uint64) []v3 {
	fb := FB{pl.Get(area + ".bin")}
	for _, z := range fb.Vector(fb.Root(), 0) {
		if fb.U64(fb.Table(z, 0), 1) != zone {
			continue
		}
		for _, h := range fb.Vector(z, 11) { // punto de vuelo
			m := fb.Table(fb.Table(h, 0), 0)
			return []v3{{float64(fb.F32(m, 0)), float64(fb.F32(m, 1)), float64(fb.F32(m, 2))}}
		}
		for _, h := range fb.Vector(z, 12) { // salida del Centro Pokemon
			m := fb.Table(fb.Table(h, 0), 0)
			return []v3{{float64(fb.F32(m, 0)), float64(fb.F32(m, 1)), float64(fb.F32(m, 2))}}
		}
		for _, h := range fb.Vector(z, 3) { // entrada
			m := fb.Table(fb.Table(h, 0), 0)
			if m >= 0 {
				return []v3{{float64(fb.F32(m, 0)), float64(fb.F32(m, 1)), float64(fb.F32(m, 2))}}
			}
		}
	}
	return nil
}

// anade puntos de aparicion (copiados de la Ruta 1, con los visibles apagados) a una zona sin ellos
func (c *Ctx) addSpawners(pl *GFPack, area string, zone uint64, pts []v3) {
	// plantilla
	tb := FB{pl.Get("a_0101.bin")}
	var tmpl int = -1
	for _, z := range tb.Vector(tb.Root(), 0) {
		if hs := tb.Vector(z, 20); len(hs) > 0 && tmpl < 0 {
			tmpl = tb.Table(hs[0], 0)
		}
	}
	subTbl := func(t int) *Tbl {
		n := NewTbl()
		for i := 0; i < tb.NumFields(t); i++ {
			if p := tb.FieldPos(t, i); p >= 0 {
				n.U32(i, le.Uint32(tb.B[p:]))
			}
		}
		return n
	}
	name := area + ".bin"
	d := pl.Get(name)
	fb := FB{d}
	for _, z := range fb.Vector(fb.Root(), 0) {
		if fb.U64(fb.Table(z, 0), 1) != zone {
			continue
		}
		at := fb.FieldPos(z, 20)
		if at < 0 {
			fatal("zona sin lista de apariciones")
		}
		vec := &VecT{}
		for _, p := range pts {
			id := tb.Table(tmpl, 0)
			ident := NewTbl().Floats(0, float32(p[0])).Floats(1, float32(p[1])).Floats(2, float32(p[2]))
			for i := 3; i < tb.NumFields(id); i++ {
				if q := tb.FieldPos(id, i); q >= 0 {
					if i >= 9 {
						ident.U64(i, le.Uint64(tb.B[q:]))
					} else {
						ident.U32(i, le.Uint32(tb.B[q:]))
					}
				}
			}
			sp := NewTbl().Ref(0, ident)
			for k := 2; k <= 5; k++ {
				s := subTbl(tb.Table(tmpl, k))
				if k == 3 {
					s.U32(0, 0) // visibles apagados
				}
				sp.Ref(k, s)
			}
			sp.U32(1, tb.U32(tmpl, 1)).U32(6, tb.U32(tmpl, 6)).U64(7, tb.U64(tmpl, 7))
			vec.Elems = append(vec.Elems, NewTbl().Ref(0, sp))
		}
		pl.Set(name, AppendFB(d, at, vec))
		return
	}
	fatal("no encuentro la zona de la ciudad en " + area)
}

var naturalMats = map[string]bool{"col_shiba": true, "col_tsuchi": true, "col_sado": true, "col_fuyodo": true, "col_yuki": true,
	"col_tsuchishiba": true, "col_karekusa": true, "col_iwaba": true}
var badMats = map[string]bool{"col_kawa": true, "col_umi": true, "col_ike": true, "col_asase": true, "col_mizutamari": true,
	"col_kusa": true, "col_yukikusa": true, "col_tsuribori": true}

func metaPos(fb FB, m int) (v v3, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	if m < 0 {
		return v, false
	}
	return v3{float64(fb.F32(m, 0)), float64(fb.F32(m, 1)), float64(fb.F32(m, 2))}, true
}

// busca un sitio para la hierba: suelo plano y natural, lejos del Centro Pokemon, puertas y punto de vuelo
func (c *Ctx) citySite(pl *GFPack, area string, zone uint64, fl *floorIdx) (v3, float64, bool) {
	fb := FB{pl.Get(area + ".bin")}
	var avoid, npcs []v3
	for _, z := range fb.Vector(fb.Root(), 0) {
		if fb.U64(fb.Table(z, 0), 1) != zone {
			continue
		}
		for _, fi := range []int{11, 12, 3} {
			for _, h := range fb.Vector(z, fi) {
				if p, ok := metaPos(fb, fb.Table(fb.Table(h, 0), 0)); ok {
					avoid = append(avoid, p)
				}
			}
		}
		for _, fi := range []int{13, 16, 8} {
			for _, h := range fb.Vector(z, fi) {
				func() {
					defer func() { recover() }()
					if p, ok := metaPos(fb, fb.Table(fb.Table(h, 0), 0)); ok {
						npcs = append(npcs, p)
					}
				}()
			}
		}
	}
	if len(avoid) == 0 {
		return v3{}, 0, false
	}
	anchor := avoid[0]
	for _, opt := range []struct{ half, av, dy float64 }{{250, 1100, 1500}, {175, 800, 3000}} {
		bestS := -1e18
		var best v3
		found := false
		for gx := -50; gx <= 50; gx++ {
			for gz := -50; gz <= 50; gz++ {
				x, zz := anchor[0]+float64(gx)*100, anchor[2]+float64(gz)*100
				if math.Hypot(x-anchor[0], zz-anchor[2]) > 5000 {
					continue
				}
				near := false
				for _, p := range avoid {
					if math.Hypot(x-p[0], zz-p[2]) < opt.av {
						near = true
						break
					}
				}
				if near {
					continue
				}
				hs := fl.query(x, zz, anchor[1], opt.dy)
				if len(hs) == 0 {
					continue
				}
				y0 := hs[0].y
				for _, h := range hs {
					if math.Abs(h.y-anchor[1]) < math.Abs(y0-anchor[1]) {
						y0 = h.y
					}
				}
				ok, nat := true, 0
				ymin, ymax := 1e18, -1e18
				for _, dx := range []float64{-opt.half, 0, opt.half} {
					for _, dz := range []float64{-opt.half, 0, opt.half} {
						q := fl.query(x+dx, zz+dz, y0, 60)
						if len(q) == 0 {
							ok = false
							break
						}
						for _, h := range q {
							if badMats[h.mat] {
								ok = false
							}
						}
						if naturalMats[q[0].mat] {
							nat++
						}
						ymin, ymax = math.Min(ymin, q[0].y), math.Max(ymax, q[0].y)
					}
					if !ok {
						break
					}
				}
				if !ok || ymax-ymin > 40 {
					continue
				}
				npcd := 3000.0
				for _, p := range npcs {
					npcd = math.Min(npcd, math.Hypot(x-p[0], zz-p[2]))
				}
				reach := 0.0
				if npcd < 2500 {
					reach = 1
				}
				s := float64(nat)*2 + reach*3 - (ymax-ymin)/20 - math.Hypot(x-anchor[0], zz-anchor[2])/2500
				if s > bestS {
					bestS, best, found = s, v3{x, y0, zz}, true
				}
			}
		}
		if found {
			return best, opt.half * 2, true
		}
	}
	return v3{}, 0, false
}

// sitio valido mas cercano a un punto elegido a mano
func manualSite(fl *floorIdx, tx, tz, half float64) (v3, bool) {
	best, bd := v3{}, 1e18
	for gx := -20; gx <= 20; gx++ {
		for gz := -20; gz <= 20; gz++ {
			x, zz := tx+float64(gx)*50, tz+float64(gz)*50
			for _, h0 := range fl.query(x, zz, 0, 5000) {
				ok := true
				ymin, ymax := 1e18, -1e18
				for _, dx := range []float64{-half, 0, half} {
					for _, dz := range []float64{-half, 0, half} {
						q := fl.query(x+dx, zz+dz, h0.y, 60)
						if len(q) == 0 {
							ok = false
							break
						}
						for _, h := range q {
							if badMats[h.mat] {
								ok = false
							}
						}
						ymin, ymax = math.Min(ymin, q[0].y), math.Max(ymax, q[0].y)
					}
					if !ok {
						break
					}
				}
				if ok && ymax-ymin < 40 {
					if d := math.Hypot(x-tx, zz-tz); d < bd {
						bd, best = d, v3{x, h0.y, zz}
					}
				}
			}
		}
	}
	return best, bd < 1e17
}

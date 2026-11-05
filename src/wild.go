package main

import "fmt"

type encSlot struct {
	prob uint8
	sp   int32
	form uint8
}
type encSub struct {
	min, max uint8
	slots    []encSlot
}
type encTable struct {
	zone uint64
	subs []encSub
}
type encArchive struct {
	f0     uint32
	tables []*encTable
}

func parseEnc(d []byte) *encArchive {
	fb := FB{d}
	r := fb.Root()
	a := &encArchive{f0: fb.U32(r, 0)}
	for _, t := range fb.Vector(r, 1) {
		et := &encTable{zone: fb.U64(t, 0)}
		for _, s := range fb.Vector(t, 1) {
			es := encSub{min: fb.U8(s, 0), max: fb.U8(s, 1)}
			for _, x := range fb.Vector(s, 2) {
				es.slots = append(es.slots, encSlot{fb.U8(x, 0), int32(fb.U32(x, 1)), fb.U8(x, 2)})
			}
			et.subs = append(et.subs, es)
		}
		a.tables = append(a.tables, et)
	}
	return a
}

func buildEnc(a *encArchive) []byte {
	tv := &VecT{}
	for _, t := range a.tables {
		sv := &VecT{}
		for _, s := range t.subs {
			slv := &VecT{}
			for _, x := range s.slots {
				slv.Elems = append(slv.Elems, NewTbl().U32(1, uint32(x.sp)).U8(0, x.prob).U8(2, x.form))
			}
			sv.Elems = append(sv.Elems, NewTbl().Ref(2, slv).U8(0, s.min).U8(1, s.max))
		}
		tv.Elems = append(tv.Elems, NewTbl().U64(0, t.zone).Ref(1, sv))
	}
	return BuildFB(NewTbl().U32(0, a.f0).Ref(1, tv))
}

var zoneNameOf = map[uint64]string{}

func zoneHash(name string, wantSymbol bool) uint64 {
	for _, z := range zoneNames {
		t := zoneTypes[z.h]
		isSym := len(t) >= 6 && t[:6] == "Symbol"
		if z.name == name && isSym == wantSymbol {
			return z.h
		}
	}
	panic("zona desconocida: " + name)
}

type dlcZone struct{ area, name, symName string }

var dlcCaves = []dlcZone{
	{"a_wr0201", "Brawlers' Cave", "Brawlers' Cave"}, {"a_wr0201", "Courageous Cavern", "Courageous Cavern"},
	{"a_wr0201", "Warm-Up Tunnel", "Warm-up Tunnel"}, {"a_wr0301", "Roaring-Sea Caves", "Roaring-Sea Caves"},
	{"a_wr0301", "Tunnel to the Top", "Tunnel to the Top"}, {"a_wr0301", "Lakeside Cave", "Lakeside Cave"},
	{"a_wr0201", "Potbottom Desert", "Potbottom Desert"}, {"a_wr0301", "Path to the Peak", "Path to the Peak"},
}

func (c *Ctx) randomizeWild(pc *poolCache) (dt *GFPack, wildByLevel map[int][]int) {
	for _, z := range zoneNames {
		if _, ok := zoneNameOf[z.h]; !ok {
			zoneNameOf[z.h] = z.name
		}
	}
	var err error
	dt, err = ReadGFPack(c.read("bin/archive/field/resident/data_table.gfpak"))
	if err != nil {
		fatal(err.Error())
	}
	hn := fmt.Sprintf("encount_%c.bin", c.game)
	sn := fmt.Sprintf("encount_symbol_%c.bin", c.game)
	H := parseEnc(dt.Get(hn))
	S := parseEnc(dt.Get(sn))
	wildByLevel = map[int][]int{}
	for _, a := range []*encArchive{H, S} {
		for _, t := range a.tables {
			for si := range t.subs {
				s := &t.subs[si]
				for k := range s.slots {
					if s.slots[k].prob > 0 {
						sp := pc.pick(int(s.max), true)
						s.slots[k].sp = int32(sp)
						s.slots[k].form = 0
						wildByLevel[int(s.max)] = append(wildByLevel[int(s.max)], sp)
					}
				}
			}
		}
	}
	// tablas ocultas nuevas para cuevas/zonas sin hierba (con las especies ya randomizadas de su propia zona)
	Ht := map[uint64]*encTable{}
	for _, t := range H.tables {
		Ht[t.zone] = t
	}
	St := map[uint64]*encTable{}
	for _, t := range S.tables {
		St[t.zone] = t
	}
	for _, z := range dlcCaves {
		zid := zoneHash(z.name, false)
		sym := St[zoneHash(z.symName, true)]
		if t, ok := Ht[zid]; ok && hasSlots(t) {
			continue
		}
		var subs []encSub
		for _, s := range sym.subs {
			ns := encSub{min: s.min, max: s.max, slots: append([]encSlot(nil), s.slots...)}
			for len(ns.slots) < 10 {
				ns.slots = append(ns.slots, encSlot{})
			}
			subs = append(subs, ns)
		}
		for len(subs) < 11 {
			subs = append(subs, encSub{slots: make([]encSlot, 10)})
		}
		if t, ok := Ht[zid]; ok {
			t.subs = subs
		} else {
			nt := &encTable{zone: zid, subs: subs}
			H.tables = append(H.tables, nt)
			Ht[zid] = nt
		}
	}
	if cityTest() {
		c.cityTables(H, pc)
		for _, t := range H.tables {
			Ht[t.zone] = t
		}
	}
	dt.Set(hn, buildEnc(H))
	dt.Set(sn, buildEnc(S))
	// log: clima normal
	c.logf("\n=== POKEMON SALVAJES (clima normal / pesca) ===")
	for _, t := range H.tables {
		if len(t.subs) == 0 {
			continue
		}
		line := ""
		for _, x := range t.subs[0].slots {
			if x.prob > 0 {
				line += fmt.Sprintf("%s %d%%, ", c.name(int(x.sp)), x.prob)
			}
		}
		fish := ""
		if len(t.subs) > 10 {
			for _, x := range t.subs[10].slots {
				if x.prob > 0 {
					fish += c.name(int(x.sp)) + ", "
				}
			}
		}
		c.logf("%s (nv. %d-%d): %s | pesca: %s", zoneNameOf[t.zone], t.subs[0].min, t.subs[0].max, line, fish)
	}
	c.hiddenOK = map[uint64]bool{}
	for z, t := range Ht {
		if hasSlots(t) {
			c.hiddenOK[z] = true
		}
	}
	return dt, wildByLevel
}

func hasSlots(t *encTable) bool {
	for i, s := range t.subs {
		if i >= 9 {
			break
		}
		for _, x := range s.slots {
			if x.prob > 0 {
				return true
			}
		}
	}
	return false
}

// apaga todos los Pokemon visibles (bloque Field_03 de cada punto de aparicion)
func (c *Ctx) disableSymbols(pl *GFPack) {
	off := 0
	for _, f := range pl.folders {
		for _, e := range f.files {
			fb := FB{pl.Files[e[1]]}
			if len(fb.B) < 8 {
				continue
			}
			func() {
				defer func() { recover() }()
				for _, z := range fb.Vector(fb.Root(), 0) {
					zid := fb.U64(fb.Table(z, 0), 1)
					if !c.hiddenOK[zid] {
						continue
					}
					for _, h := range fb.Vector(z, 20) {
						o := fb.Table(h, 0)
						if fb.SetU32(fb.Table(o, 3), 0, 0) {
							off++
						}
					}
				}
			}()
		}
	}
	c.logf("Pokemon visibles desactivados: %d puntos de aparicion", off)
}

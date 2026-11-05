package main

import (
	"fmt"
	"sort"
)

// especies de historia que no se tocan en regalos (Kubfu, Zacian/Zamazenta, Calyrex y corceles)
var storyGift = map[int]bool{891: true, 888: true, 889: true, 898: true, 896: true, 897: true}

// escenarios de estaticos con combate de historia guionizado
var storyScenario = map[uint32]bool{4: true, 5: true, 6: true, 7: true, 13: true, 17: true, 18: true, 19: true}

func (c *Ctx) pickStarters() [3]int {
	var cands []int
	for sp := 1; sp <= 898; sp++ {
		if c.present(sp) && !c.hasPrev(sp) && c.depth(sp) >= 2 {
			cands = append(cands, sp)
		}
	}
	var r [3]int
	used := map[int]bool{}
	for i := 0; i < 3; i++ {
		for {
			s := cands[c.rng.Intn(len(cands))]
			if !used[s] {
				used[s] = true
				r[i] = s
				break
			}
		}
	}
	return r
}

func (c *Ctx) line(sp int) []int {
	l := []int{sp}
	for cur := sp; ; {
		n := c.next(cur)
		if len(n) == 0 || len(l) >= 3 {
			break
		}
		cur = n[0]
		l = append(l, cur)
	}
	return l
}

// devuelve el mapa de especies de la linea inicial original -> nueva (para el rival)
func (c *Ctx) randomizeGifts(pc *poolCache, pl *GFPack) map[int]int {
	st := c.pickStarters()
	origBase := []int{810, 813, 816}
	starterMap := map[int]int{}
	for i, ob := range origBase {
		nl := c.line(st[i])
		for k := 0; k < 3; k++ {
			if k < len(nl) {
				starterMap[ob+k] = nl[k]
			} else {
				starterMap[ob+k] = nl[len(nl)-1]
			}
		}
	}
	c.logf("\n=== INICIALES ===\n%s (en lugar de Grookey), %s (en lugar de Scorbunny), %s (en lugar de Sobble)",
		c.name(st[0]), c.name(st[1]), c.name(st[2]))
	// regalos
	d := append([]byte(nil), c.read("bin/script_event_data/add_poke.bin")...)
	fb := FB{d}
	c.logf("\n=== REGALOS / EVENTOS ===")
	for _, g := range fb.Vector(fb.Root(), 0) {
		sp := int(fb.U32(g, 9))
		lv := int(fb.U8(g, 8))
		var ns int
		switch {
		case sp == 810 || sp == 813 || sp == 816:
			ns = starterMap[sp]
		case storyGift[sp]:
			continue
		default:
			if fb.U32(g, 0) != 0 { // huevo: forma base
				ns = pc.pick(1, true)
			} else {
				ns = pc.pick(lv, true)
			}
		}
		fb.SetU32(g, 9, uint32(ns))
		fb.SetU8(g, 1, 0)
		fb.SetU8(g, 6, 0)
		fb.SetU32(g, 27, 0)
		c.logf("%s nv.%d -> %s", c.name(sp), lv, c.name(ns))
	}
	c.put("bin/script_event_data/add_poke.bin", d)
	// modelos de los iniciales en la mesa del principio (a_0101)
	pd := pl.Get("a_0101.bin")
	pfb := FB{pd}
	for _, z := range pfb.Vector(pfb.Root(), 0) {
		for _, cr := range pfb.Vector(z, 2) {
			sp := int(pfb.U32(cr, 2))
			if sp == 810 || sp == 813 || sp == 816 {
				pfb.SetU32(cr, 2, uint32(starterMap[sp]))
				pfb.SetU32(cr, 3, 0)
			}
		}
	}
	return starterMap
}

func (c *Ctx) randomizeStatics(pc *poolCache) {
	d := append([]byte(nil), c.read("bin/script_event_data/event_encount_data.bin")...)
	fb := FB{d}
	c.logf("\n=== POKEMON ESTATICOS ===")
	for _, s := range fb.Vector(fb.Root(), 0) {
		if storyScenario[fb.U32(s, 16)] {
			continue
		}
		sp := int(fb.U32(s, 17))
		lv := int(fb.U8(s, 15))
		ns := pc.pick(lv, true)
		fb.SetU32(s, 17, uint32(ns))
		fb.SetU8(s, 8, 0)
		fb.SetU8(s, 13, 0)
		mv := c.movesAt(ns, lv)
		for i := 0; i < 4; i++ {
			fb.SetU32(s, 28+i, uint32(mv[i]))
		}
		c.logf("%s nv.%d -> %s", c.name(sp), lv, c.name(ns))
	}
	c.put("bin/script_event_data/event_encount_data.bin", d)
}

func (c *Ctx) randomizeTrades(pc *poolCache, wildByLevel map[int][]int) {
	d := append([]byte(nil), c.read("bin/script_event_data/field_trade.bin")...)
	fb := FB{d}
	var levels []int
	for l := range wildByLevel {
		levels = append(levels, l)
	}
	sort.Ints(levels)
	c.logf("\n=== INTERCAMBIOS ===")
	for _, t := range fb.Vector(fb.Root(), 0) {
		lv := int(fb.U8(t, 7))
		sp := int(fb.U32(t, 8))
		req := int(fb.U32(t, 18))
		ns := pc.pick(lv, true)
		// lo que pide: una especie que realmente aparece en estado salvaje hasta ese nivel
		var avail []int
		for _, l := range levels {
			if l <= lv+3 {
				avail = append(avail, wildByLevel[l]...)
			}
		}
		if len(avail) == 0 {
			avail = wildByLevel[levels[0]]
		}
		nreq := avail[c.rng.Intn(len(avail))]
		fb.SetU32(t, 8, uint32(ns))
		fb.SetU8(t, 0, 0)
		fb.SetU8(t, 5, 0)
		fb.SetU32(t, 18, uint32(nreq))
		fb.SetU8(t, 17, 0)
		for i := 0; i < 4; i++ {
			fb.SetU16(t, 31+i, 0)
		}
		c.logf("Pide %s (antes %s) y da %s (antes %s)", c.name(nreq), c.name(req), c.name(ns), c.name(sp))
	}
	c.put("bin/script_event_data/field_trade.bin", d)
}

// objetos del suelo, MTs y objetos ocultos
func (c *Ctx) randomizeFieldItems(pl *GFPack) {
	normal := map[int]bool{}
	for _, i := range heldItems {
		normal[i] = true
	}
	tm := map[int]bool{}
	for _, i := range tmItems {
		tm[i] = true
	}
	for _, i := range trItems {
		tm[i] = true
	}
	var heldOK, tmOK []int
	for _, i := range heldItems {
		if _, ok := c.itemHash[i]; ok {
			heldOK = append(heldOK, i)
		}
	}
	for _, i := range tmItems {
		if _, ok := c.itemHash[i]; ok {
			tmOK = append(tmOK, i)
		}
	}
	nf, nh := 0, 0
	for _, f := range pl.folders {
		for _, e := range f.files {
			fb := FB{pl.Files[e[1]]}
			func() {
				defer func() { recover() }()
				for _, z := range fb.Vector(fb.Root(), 0) {
					if _, ok := zoneNameOf[fb.U64(fb.Table(z, 0), 1)]; !ok {
						continue
					}
					for _, h := range fb.Vector(z, 6) {
						it := fb.Table(h, 0)
						model := fb.Str(it, 2)
						p, n := fb.VecRaw(it, 6)
						if n == 0 {
							continue
						}
						cur, ok := c.hashItem[le.Uint64(fb.B[p:])]
						if !ok {
							continue
						}
						switch {
						case containsBytes([]byte(model), "itemyel") && tm[cur]:
							le.PutUint64(fb.B[p:], c.itemHash[tmOK[c.rng.Intn(len(tmOK))]])
							nf++
						case containsBytes([]byte(model), "itemred") && normal[cur]:
							le.PutUint64(fb.B[p:], c.itemHash[heldOK[c.rng.Intn(len(heldOK))]])
							if q, k := fb.VecRaw(it, 7); k > 0 {
								le.PutUint32(fb.B[q:], 1)
							}
							nf++
						}
					}
					for _, h := range fb.Vector(z, 19) {
						it := fb.Table(h, 0)
						ch := fb.Vector(it, 2)
						if len(ch) == 0 {
							continue
						}
						cur, ok := c.hashItem[fb.U64(ch[0], 0)]
						if !ok || !normal[cur] {
							continue
						}
						fb.SetU64(ch[0], 0, c.itemHash[heldOK[c.rng.Intn(len(heldOK))]])
						fb.SetU32(ch[0], 1, 100)
						fb.SetU32(ch[0], 2, 1)
						nh++
					}
				}
			}()
		}
	}
	c.logf("\nObjetos del mapa randomizados: %d visibles/MTs, %d ocultos", nf, nh)
}

var _ = fmt.Sprint

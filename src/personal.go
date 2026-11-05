package main

// Movimientos por nivel, habilidades y tutores.

var bannedAbilities = map[int]bool{
	25: true, // Superguarda (pedido)
	// habilidades que cambian de forma: pueden colgar el juego en especies que no son las suyas
	59: true, 121: true, 122: true, 161: true, 176: true, 197: true, 208: true, 209: true,
	210: true, 211: true, 225: true, 241: true, 248: true, 258: true,
}

var bannedMoves = map[int]bool{0: true, 165: true} // nada, Forcejeo

func (c *Ctx) randomizeLearnsets() {
	var all []int
	byType := map[int][]int{}
	for _, m := range c.moves {
		if !m.usable || bannedMoves[m.id] || m.id >= len(c.moveName) {
			continue
		}
		all = append(all, m.id)
		byType[m.typ] = append(byType[m.typ], m.id)
	}
	dmg := map[int]bool{}
	for _, m := range c.moves {
		if m.usable && m.power > 1 && m.cat != 2 {
			dmg[m.id] = true
		}
	}
	n := len(c.learn) / LS
	for i := 0; i < n && i*PS < len(c.personal); i++ {
		d := c.learn[i*LS : (i+1)*LS]
		t1, t2 := int(c.personal[i*PS+6]), int(c.personal[i*PS+7])
		used := map[int]bool{}
		first := true
		for k := 0; k < LS; k += 4 {
			if d[k+3] == 0xFF {
				break
			}
			var mv int
			for tries := 0; tries < 50; tries++ {
				if c.rng.Float64() < 0.10 {
					t := t1
					if t2 != t1 && c.rng.Intn(2) == 1 {
						t = t2
					}
					l := byType[t]
					mv = l[c.rng.Intn(len(l))]
				} else {
					mv = all[c.rng.Intn(len(all))]
				}
				if used[mv] {
					continue
				}
				if first && !dmg[mv] { // el primer movimiento siempre hace dano
					continue
				}
				break
			}
			used[mv] = true
			first = false
			le.PutUint16(d[k:], uint16(mv))
		}
	}
	c.logf("Movimientos por nivel: randomizados (10%% del mismo tipo, el primero siempre ofensivo)")
}

func (c *Ctx) randomizeAbilities() {
	var ab []int
	for a := 1; a <= 267; a++ {
		if !bannedAbilities[a] {
			ab = append(ab, a)
		}
	}
	n := len(c.personal) / PS
	for i := 1; i < n; i++ {
		p := c.P(i)
		for _, off := range []int{0x18, 0x1A, 0x1C} {
			if le.Uint16(p[off:]) != 0 {
				le.PutUint16(p[off:], uint16(ab[c.rng.Intn(len(ab))]))
			}
		}
	}
	c.logf("Habilidades: randomizadas (sin Superguarda ni habilidades de cambio de forma)")
}

func (c *Ctx) randomizeTutors() {
	n := len(c.personal) / PS
	type bitpos struct{ off, bit int }
	var bits []bitpos
	for b := 0; b < 8; b++ {
		bits = append(bits, bitpos{0x38, b})
	}
	for b := 0; b < 32; b++ {
		bits = append(bits, bitpos{0xA8, b})
	}
	for _, bp := range bits {
		set, tot := 0, 0
		for i := 1; i < n; i++ {
			if !c.present(i) && i <= 898 {
				continue
			}
			tot++
			if c.P(i)[bp.off+bp.bit/8]&(1<<(bp.bit%8)) != 0 {
				set++
			}
		}
		if set == 0 || tot == 0 {
			continue
		}
		prob := float64(set) / float64(tot)
		for i := 1; i < n; i++ {
			p := c.P(i)
			byteI := bp.off + bp.bit/8
			if c.rng.Float64() < prob {
				p[byteI] |= 1 << (bp.bit % 8)
			} else {
				p[byteI] &^= 1 << (bp.bit % 8)
			}
		}
	}
	c.logf("Tutores de movimientos: compatibilidades randomizadas")
}

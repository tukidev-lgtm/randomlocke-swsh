package main

import "fmt"

// Evoluciones por intercambio (simple, con objeto, Karrablast/Shelmet) -> subir a nivel 30
// (o un nivel por encima de la evolucion anterior si esta es mayor).
func (c *Ctx) tradeEvolutions() {
	n := len(c.personal) / PS
	changed := 0
	c.logf("\n=== EVOLUCIONES POR INTERCAMBIO -> NIVEL ===")
	for i := 0; i < n; i++ {
		rel := fmt.Sprintf("bin/pml/evolution/evo_%03d.bin", i)
		d, err := readOpt(c.path(rel))
		if err != nil || len(d) == 0 {
			continue
		}
		mod := false
		for k := 0; k+8 <= len(d); k += 8 {
			m := le.Uint16(d[k:])
			if m != 5 && m != 6 && m != 7 {
				continue
			}
			lv := 30
			// si el propio Pokemon se consigue por nivel mayor, evoluciona 1 nivel despues
			if p, ok := c.prev[i]; ok && i <= 898 {
				if pd, err := readOpt(c.path(fmt.Sprintf("bin/pml/evolution/evo_%03d.bin", p))); err == nil {
					for q := 0; q+8 <= len(pd); q += 8 {
						if int(le.Uint16(pd[q+4:])) == i && int(pd[q+7]) >= lv {
							lv = int(pd[q+7]) + 1
						}
					}
				}
			}
			le.PutUint16(d[k:], 4)   // LevelUp
			le.PutUint16(d[k+2:], 0) // sin objeto
			d[k+7] = byte(lv)
			mod = true
			changed++
			if i <= 898 {
				c.logf("%s -> %s: nivel %d", c.name(i), c.name(int(le.Uint16(d[k+4:]))), lv)
			}
		}
		if mod {
			c.put(rel, d)
		}
	}
	c.logf("Evoluciones por intercambio convertidas: %d", changed)
}

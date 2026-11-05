package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var signature = map[string]bool{"Paul": true, "Berto": true, "Roxy": true, "Percy": true, "Cathy": true, "Naboru": true, "Judith": true,
	"Alistair": true, "Sally": true, "Morris": true, "Mel": true, "Nerio": true, "Roy": true, "Olivia": true, "Rose": true, "Lionel": true,
	"Mostaz": true, "Dreo": true, "Sófora": true, "Peony": true, "Tizonio": true, "Dargo": true,
	// nombres en ingles por si el volcado no tiene espanol
	"Hop": true, "Bede": true, "Marnie": true, "Milo": true, "Nessa": true, "Kabu": true, "Bea": true, "Allister": true, "Opal": true,
	"Gordie": true, "Melony": true, "Piers": true, "Raihan": true, "Oleana": true, "Leon": true, "Mustard": true, "Klara": true,
	"Avery": true, "Sordward": true, "Shielbert": true}

func targetSize(ace int) int {
	switch {
	case ace <= 9:
		return 0
	case ace <= 16:
		return 3
	case ace <= 21:
		return 4
	case ace <= 26:
		return 5
	}
	return 6
}

// starterMap: especie original de la linea inicial -> especie nueva en la misma etapa
func (c *Ctx) randomizeTrainers(pc *poolCache, starterMap map[int]int) {
	files, _ := filepath.Glob(c.path("bin/trainer/trainer_data/*.bin"))
	sort.Strings(files)
	aiCount := 0
	c.logf("\n=== ENTRENADORES IMPORTANTES ===")
	for _, f := range files {
		base := filepath.Base(f)
		var id int
		fmt.Sscanf(base, "trainer_data_%d.bin", &id)
		d, _ := os.ReadFile(f)
		pf := c.path(fmt.Sprintf("bin/trainer/trainer_poke/trainer_poke_%03d.bin", id))
		p, err := os.ReadFile(pf)
		if err != nil {
			continue
		}
		num, mode := int(d[3]), int(d[2])
		if num == 0 {
			continue
		}
		ai := le.Uint32(d[0xC:])
		newAI := (ai | 0b111) &^ 0x40
		if newAI != ai {
			aiCount++
		}
		le.PutUint32(d[0xC:], newAI)
		var team [][]byte
		for o := 0; o+0x20 <= len(p); o += 0x20 {
			team = append(team, append([]byte(nil), p[o:o+0x20]...))
		}
		name := ""
		if id < len(c.trName) {
			name = c.trName[id]
		}
		minLv, ace := 999, 0
		for _, t := range team {
			l := int(le.Uint16(t[0x0A:]))
			if l < minLv {
				minLv = l
			}
			if l > ace {
				ace = l
			}
		}
		// randomizar cada Pokemon
		for _, t := range team {
			lv := int(le.Uint16(t[0x0A:]))
			orig := int(le.Uint16(t[0x0C:]))
			sp, ok := starterMap[orig]
			if !ok {
				sp = pc.pick(lv, false)
			}
			setTrainerMon(c, t, sp, lv)
		}
		// equipos progresivos para los importantes
		if signature[name] && num >= 2 && (mode == 0 || id == 144) {
			if tg := targetSize(ace); tg > len(team) {
				for len(team) < tg {
					nt := append([]byte(nil), team[0]...)
					setTrainerMon(c, nt, pc.pick(minLv, false), minLv)
					nt[0x10], nt[0x11] = 0, 0
					team = append(team[:len(team)-1], append([][]byte{nt}, team[len(team)-1])...)
				}
				d[3] = byte(len(team))
			}
		}
		if signature[name] && ace > 9 {
			var names []string
			for _, t := range team {
				names = append(names, fmt.Sprintf("%s nv.%d", c.name(int(le.Uint16(t[0x0C:]))), le.Uint16(t[0x0A:])))
			}
			c.logf("[%03d] %s: %s", id, name, strings.Join(names, ", "))
		}
		c.put("bin/trainer/trainer_data/"+base, d)
		var out []byte
		for _, t := range team {
			out = append(out, t...)
		}
		c.put(fmt.Sprintf("bin/trainer/trainer_poke/trainer_poke_%03d.bin", id), out)
	}
	c.logf("IA mejorada (Basic+Strong+Expert, sin cambiar de Pokemon) en %d entrenadores", aiCount)
}

func setTrainerMon(c *Ctx, t []byte, sp, lv int) {
	le.PutUint16(t[0x0A:], uint16(lv))
	le.PutUint16(t[0x0C:], uint16(sp))
	le.PutUint16(t[0x0E:], 0) // forma
	mv := c.movesAt(sp, lv)
	for i := 0; i < 4; i++ {
		le.PutUint16(t[0x12+2*i:], mv[i])
	}
	t[0x09] = 0                             // sin Gigantamax
	t[0] &^= 0x30                           // habilidad 1 (las habilidades ya estan randomizadas)
	iv := le.Uint32(t[0x1C:]) &^ 0x40000000 // sin shiny
	le.PutUint32(t[0x1C:], iv)
}

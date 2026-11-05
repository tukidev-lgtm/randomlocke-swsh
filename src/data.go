package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Ctx struct {
	rom, out string
	game     byte // 'k' Espada, 't' Escudo
	seed     int64
	rng      *rand.Rand
	log      strings.Builder

	personal []byte
	learn    []byte
	evoNext  [][]int // por indice personal: especies a las que evoluciona
	prev     map[int]int
	moves    []moveInfo
	monName  []string
	moveName []string
	itemName []string
	trName   []string
	itemHash map[int]uint64
	hashItem map[uint64]int
	outFiles map[string][]byte
	hiddenOK map[uint64]bool
}

type moveInfo struct {
	id     int
	usable bool
	typ    int
	power  int
	cat    int
}

const PS = 0xB0
const LS = 0x104

func (c *Ctx) path(rel string) string { return filepath.Join(c.rom, filepath.FromSlash(rel)) }
func (c *Ctx) read(rel string) []byte {
	b, err := os.ReadFile(c.path(rel))
	if err != nil {
		fatal("No se pudo leer " + rel + ": " + err.Error())
	}
	return b
}
func (c *Ctx) put(rel string, d []byte) { c.outFiles[rel] = d }
func (c *Ctx) logf(f string, a ...interface{}) {
	c.log.WriteString(fmt.Sprintf(f, a...))
	c.log.WriteString("\n")
}

func readText(d []byte) []string {
	n := int(le.Uint16(d[2:]))
	sdo := int(le.Uint32(d[0xC:]))
	out := make([]string, n)
	key := uint16(0x7C89)
	for i := 0; i < n; i++ {
		off := int(int32(le.Uint32(d[sdo+4+8*i:])))
		ln := int(le.Uint16(d[sdo+8+8*i:]))
		k := key
		var sb strings.Builder
		for j := 0; j < ln; j++ {
			ch := le.Uint16(d[sdo+off+2*j:]) ^ k
			k = k<<3 | k>>13
			if ch == 0 {
				break
			}
			if ch >= 0x20 {
				sb.WriteRune(rune(ch))
			}
		}
		out[i] = sb.String()
		key += 0x2983
	}
	return out
}

func (c *Ctx) loadData() {
	c.personal = c.read("bin/pml/personal/personal_total.bin")
	c.learn = c.read("bin/pml/waza_oboe/wazaoboe_total.bin")
	n := len(c.personal) / PS
	c.evoNext = make([][]int, n)
	c.prev = map[int]int{}
	for i := 0; i < n; i++ {
		d, err := os.ReadFile(c.path(fmt.Sprintf("bin/pml/evolution/evo_%03d.bin", i)))
		if err != nil {
			continue
		}
		for k := 0; k+8 <= len(d); k += 8 {
			sp := int(le.Uint16(d[k+4:]))
			if sp > 0 && le.Uint16(d[k:]) != 0 {
				c.evoNext[i] = append(c.evoNext[i], sp)
				if i <= 898 {
					if _, ok := c.prev[sp]; !ok {
						c.prev[sp] = i
					}
				}
			}
		}
	}
	// evoluciones especiales que no estan en los archivos (Shedinja sale de Nincada)
	if _, ok := c.prev[292]; !ok {
		c.prev[292] = 290
	}
	files, _ := filepath.Glob(c.path("bin/pml/waza/waza*.wazabin"))
	sort.Strings(files)
	for _, f := range files {
		b, _ := os.ReadFile(f)
		fb := FB{b}
		r := fb.Root()
		c.moves = append(c.moves, moveInfo{id: int(fb.U32(r, 1)), usable: fb.U8(r, 2) != 0, typ: int(fb.U8(r, 3)), cat: int(fb.U8(r, 5)), power: int(fb.U8(r, 6))})
	}
	lang := "Spanish"
	if _, err := os.Stat(c.path("bin/message/Spanish/common/monsname.dat")); err != nil {
		lang = "English"
	}
	c.monName = readText(c.read("bin/message/" + lang + "/common/monsname.dat"))
	c.moveName = readText(c.read("bin/message/" + lang + "/common/wazaname.dat"))
	c.itemName = readText(c.read("bin/message/" + lang + "/common/itemname.dat"))
	c.trName = readText(c.read("bin/message/" + lang + "/common/trname.dat"))
	ih := c.read("bin/pml/item/item_hash_to_index.dat")
	cnt := int(le.Uint32(ih))
	c.itemHash = map[int]uint64{}
	c.hashItem = map[uint64]int{}
	for i := 0; i < cnt; i++ {
		h := le.Uint64(ih[4+16*i:])
		k := int(int32(le.Uint32(ih[12+16*i:])))
		if k == 0 {
			continue
		}
		if _, ok := c.itemHash[k]; !ok {
			if _, ok2 := c.hashItem[h]; !ok2 {
				c.itemHash[k] = h
				c.hashItem[h] = k
			}
		}
	}
}

func (c *Ctx) P(i int) []byte { return c.personal[i*PS : (i+1)*PS] }
func (c *Ctx) present(sp int) bool {
	return sp > 0 && sp < len(c.personal)/PS && (c.P(sp)[0x21]>>6)&1 == 1
}
func (c *Ctx) bst(sp int) int {
	p := c.P(sp)
	return int(p[0]) + int(p[1]) + int(p[2]) + int(p[3]) + int(p[4]) + int(p[5])
}
func (c *Ctx) types(sp int) (int, int) { p := c.P(sp); return int(p[6]), int(p[7]) }
func (c *Ctx) name(sp int) string {
	if sp >= 0 && sp < len(c.monName) {
		return c.monName[sp]
	}
	return fmt.Sprint(sp)
}
func (c *Ctx) next(sp int) []int {
	var r []int
	for _, s := range c.evoNext[sp] {
		if s <= 898 && c.present(s) {
			r = append(r, s)
		}
	}
	return r
}
func (c *Ctx) hasPrev(sp int) bool { p, ok := c.prev[sp]; return ok && c.present(p) }
func (c *Ctx) depth(sp int) int {
	best := 0
	for _, s := range c.next(sp) {
		if d := 1 + c.depth(s); d > best {
			best = d
		}
	}
	return best
}
func (c *Ctx) stage(sp int) int {
	st := 1
	for cur := sp; c.hasPrev(cur); cur = c.prev[cur] {
		st++
		if st > 4 {
			break
		}
	}
	return st
}

// ---------- seleccion progresiva ----------
// wild=true: pokemon salvajes/estaticos/regalos (todo permitido desde nv. 25)
// wild=false: entrenadores (solo evoluciones finales desde nv. 32)
func (c *Ctx) pool(level int, wild bool) []int {
	full := 32
	if wild {
		full = 25
	}
	var r []int
	for sp := 1; sp <= 898; sp++ {
		if !c.present(sp) {
			continue
		}
		hp, nx := c.hasPrev(sp), len(c.next(sp)) > 0
		single := !hp && !nx
		ok := false
		switch {
		case level >= full:
			ok = wild || !nx
		case level < 20:
			ok = (!hp && nx) || (single && c.bst(sp) <= 450)
		default:
			ok = (!hp && nx) || (hp && nx) || single || (hp && !nx && c.stage(sp) == 2 && c.bst(sp) <= 480)
		}
		if ok {
			r = append(r, sp)
		}
	}
	return r
}

type poolCache struct {
	c *Ctx
	m map[[2]int][]int
}

func (pc *poolCache) pick(level int, wild bool) int {
	w := 0
	if wild {
		w = 1
	}
	k := [2]int{level, w}
	p, ok := pc.m[k]
	if !ok {
		p = pc.c.pool(level, wild)
		pc.m[k] = p
	}
	return p[pc.c.rng.Intn(len(p))]
}

// ultimos 4 movimientos por nivel (de los learnsets ya randomizados)
func (c *Ctx) movesAt(sp, level int) [4]uint16 {
	d := c.learn[sp*LS : (sp+1)*LS]
	var last []uint16
	for k := 0; k < LS; k += 4 {
		if d[k+3] == 0xFF {
			break
		}
		m, l := le.Uint16(d[k:]), int(int16(le.Uint16(d[k+2:])))
		if l <= level && m > 0 {
			for i, x := range last {
				if x == m {
					last = append(last[:i], last[i+1:]...)
					break
				}
			}
			last = append(last, m)
		}
	}
	if len(last) > 4 {
		last = last[len(last)-4:]
	}
	var r [4]uint16
	copy(r[:], last)
	return r
}

func fatal(msg string) {
	fmt.Println("\nERROR:", msg)
	fmt.Println("Pulsa Enter para salir.")
	fmt.Scanln()
	os.Exit(1)
}

func readOpt(p string) ([]byte, error) { return os.ReadFile(p) }

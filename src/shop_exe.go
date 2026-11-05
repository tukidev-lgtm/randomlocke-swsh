package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const rareCandy = 50

func (c *Ctx) shopCandies() {
	d := c.read("bin/appli/shop/bin/shop_data.bin")
	fb := FB{d}
	r := fb.Root()
	items := func(inv int) []uint32 {
		p, n := fb.VecRaw(inv, 0)
		v := make([]uint32, n)
		for i := range v {
			v[i] = le.Uint32(fb.B[p+4*i:])
		}
		return v
	}
	add := func(v []uint32) []uint32 {
		for _, x := range v {
			if x == rareCandy {
				return v
			}
		}
		return append([]uint32{rareCandy}, v...)
	}
	singles := &VecT{}
	for _, t := range fb.Vector(r, 0) {
		h := fb.U64(t, 0)
		v := items(fb.Table(t, 1))
		if h == 0x1F3FF031A3A24490 {
			v = add(v)
		}
		singles.Elems = append(singles.Elems, NewTbl().U64(0, h).Ref(1, NewTbl().Ref(0, VecU32(v))))
	}
	multis := &VecT{}
	for _, t := range fb.Vector(r, 1) {
		h := fb.U64(t, 0)
		invs := &VecT{}
		for _, inv := range fb.Vector(t, 1) {
			v := items(inv)
			if h == 0x66CA73B2966BB871 {
				v = add(v)
			}
			invs.Elems = append(invs.Elems, NewTbl().Ref(0, VecU32(v)))
		}
		multis.Elems = append(multis.Elems, NewTbl().U64(0, h).Ref(1, invs))
	}
	c.put("bin/appli/shop/bin/shop_data.bin", BuildFB(NewTbl().Ref(0, singles).Ref(1, multis)))
	it := append([]byte(nil), c.read("bin/pml/item/item.dat")...)
	start := int(int32(le.Uint32(it[0x40:])))
	e := int(le.Uint16(it[0x44+2*rareCandy:]))
	le.PutUint32(it[start+e*0x30:], 0)
	c.put("bin/pml/item/item.dat", it)
	c.logf("Caramelos Raros a 0$ en todas las tiendas Pokemon")
}

// ---------- parche del Repartir Experiencia ----------

func (c *Ctx) expSharePatch() (string, []byte) {
	raw, err0 := os.ReadFile(filepath.Join(filepath.Dir(c.rom), "exefs", "main"))
	if err0 != nil {
		fatal("No se pudo leer exefs/main: " + err0.Error())
	}
	if string(raw[:4]) != "NSO0" {
		fatal("exefs/main no es un NSO")
	}
	flags := le.Uint32(raw[0xC:])
	fo := le.Uint32(raw[0x10:])
	ds := le.Uint32(raw[0x18:])
	cs := le.Uint32(raw[0x60:])
	text := raw[fo : fo+cs]
	if flags&1 != 0 {
		var err error
		text, err = lz4Decode(text, int(ds))
		if err != nil {
			fatal("no se pudo descomprimir exefs/main: " + err.Error())
		}
	}
	sig, _ := hex.DecodeString("09804039f30301aaf40300aa6900003588864039")
	i := bytes.Index(text, sig)
	if i < 0 || bytes.Index(text[i+1:], sig) >= 0 {
		c.logf("AVISO: no se encontro el codigo del Repartir Experiencia; no se desactiva")
		return "", nil
	}
	pc := i + len(sig)
	ins := le.Uint32(text[pc:])
	if ins&0xFF00001F != 0x34000008 { // cbz w8, ...
		c.logf("AVISO: instruccion inesperada; no se desactiva el Repartir Experiencia")
		return "", nil
	}
	imm := int32(ins<<8) >> 13 // imm19 con signo
	b := uint32(0x14000000) | (uint32(imm) & 0x03FFFFFF)
	off := pc + 0x100
	var ips bytes.Buffer
	ips.WriteString("PATCH")
	ips.Write([]byte{byte(off >> 16), byte(off >> 8), byte(off), 0, 4})
	ips.Write([]byte{byte(b), byte(b >> 8), byte(b >> 16), byte(b >> 24)})
	ips.WriteString("EOF")
	bid := strings.ToUpper(hex.EncodeToString(raw[0x40:0x60]))
	bid = strings.TrimRight(bid, "0")
	if len(bid)%2 == 1 {
		bid += "0"
	}
	c.logf("Repartir Experiencia desactivado (parche %s.ips, offset 0x%X)", bid, pc)
	return bid, ips.Bytes()
}

var _ = fmt.Sprint

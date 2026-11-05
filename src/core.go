package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
)

var le = binary.LittleEndian

// ---------------- LZ4 (bloque) ----------------

func lz4Decode(src []byte, size int) ([]byte, error) {
	dst := make([]byte, 0, size)
	i := 0
	for i < len(src) {
		tok := src[i]
		i++
		lit := int(tok >> 4)
		if lit == 15 {
			for {
				b := src[i]
				i++
				lit += int(b)
				if b != 255 {
					break
				}
			}
		}
		dst = append(dst, src[i:i+lit]...)
		i += lit
		if i >= len(src) {
			break
		}
		off := int(src[i]) | int(src[i+1])<<8
		i += 2
		ml := int(tok & 15)
		if ml == 15 {
			for {
				b := src[i]
				i++
				ml += int(b)
				if b != 255 {
					break
				}
			}
		}
		ml += 4
		p := len(dst) - off
		if p < 0 {
			return nil, errors.New("lz4: offset invalido")
		}
		for k := 0; k < ml; k++ {
			dst = append(dst, dst[p+k])
		}
	}
	if len(dst) != size {
		return nil, fmt.Errorf("lz4: tamano %d != %d", len(dst), size)
	}
	return dst, nil
}

func lz4Encode(src []byte) []byte {
	const minMatch = 4
	n := len(src)
	var out bytes.Buffer
	writeLen := func(l int) {
		for l >= 255 {
			out.WriteByte(255)
			l -= 255
		}
		out.WriteByte(byte(l))
	}
	emit := func(lits []byte, off, ml int, last bool) {
		ll := len(lits)
		tok := byte(0)
		if ll >= 15 {
			tok = 15 << 4
		} else {
			tok = byte(ll) << 4
		}
		if !last {
			m := ml - minMatch
			if m >= 15 {
				tok |= 15
			} else {
				tok |= byte(m)
			}
		}
		out.WriteByte(tok)
		if ll >= 15 {
			writeLen(ll - 15)
		}
		out.Write(lits)
		if !last {
			out.WriteByte(byte(off))
			out.WriteByte(byte(off >> 8))
			if ml-minMatch >= 15 {
				writeLen(ml - minMatch - 15)
			}
		}
	}
	if n < 13 {
		emit(src, 0, 0, true)
		return out.Bytes()
	}
	const hbits = 16
	table := make([]int, 1<<hbits)
	for i := range table {
		table[i] = -1
	}
	hash := func(p int) uint32 { return (le.Uint32(src[p:]) * 2654435761) >> (32 - hbits) }
	anchor := 0
	i := 0
	limit := n - 12 // ultimos 5 bytes literales y match no puede empezar en los ultimos 12
	for i < limit {
		h := hash(i)
		ref := table[h]
		table[h] = i
		if ref >= 0 && i-ref <= 65535 && le.Uint32(src[ref:]) == le.Uint32(src[i:]) {
			ml := minMatch
			for i+ml < n-5 && src[ref+ml] == src[i+ml] {
				ml++
			}
			emit(src[anchor:i], i-ref, ml, false)
			i += ml
			anchor = i
			if i < limit {
				table[hash(i-2)] = i - 2
			}
			continue
		}
		i++
	}
	emit(src[anchor:], 0, 0, true)
	return out.Bytes()
}

// ---------------- GFPack ----------------

const gfMagic = 0x4B434150584C4647

type gfFolder struct {
	hash  uint64
	pad   uint32
	files [][3]uint64 // hash, index, pad
}

type GFPack struct {
	version, reloc uint32
	hashAbs        []uint64
	folders        []gfFolder
	ft             [][8]int64 // level,type,b,sd,sc,pad,off,unused
	Files          [][]byte
}

func fnv1a64(s string) uint64 {
	h := uint64(0xCBF29CE484222645)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 0x100000001B3
	}
	return h
}

func ReadGFPack(d []byte) (*GFPack, error) {
	if le.Uint64(d) != gfMagic {
		return nil, errors.New("no es un GFPack")
	}
	p := &GFPack{version: le.Uint32(d[8:]), reloc: le.Uint32(d[12:])}
	nf := int(le.Uint32(d[16:]))
	nd := int(le.Uint32(d[20:]))
	ptrFT := int(le.Uint64(d[0x18:]))
	ptrHP := int(le.Uint64(d[0x20:]))
	for i := 0; i < nf; i++ {
		p.hashAbs = append(p.hashAbs, le.Uint64(d[ptrHP+8*i:]))
	}
	for f := 0; f < nd; f++ {
		fp := int(le.Uint64(d[0x28+8*f:]))
		fo := gfFolder{hash: le.Uint64(d[fp:]), pad: le.Uint32(d[fp+12:])}
		cnt := int(le.Uint32(d[fp+8:]))
		for i := 0; i < cnt; i++ {
			q := fp + 16 + 16*i
			fo.files = append(fo.files, [3]uint64{le.Uint64(d[q:]), uint64(le.Uint32(d[q+8:])), uint64(le.Uint32(d[q+12:]))})
		}
		p.folders = append(p.folders, fo)
	}
	for i := 0; i < nf; i++ {
		q := ptrFT + 0x18*i
		e := [8]int64{int64(le.Uint16(d[q:])), int64(d[q+2]), int64(d[q+3]), int64(int32(le.Uint32(d[q+4:]))), int64(int32(le.Uint32(d[q+8:]))),
			int64(int32(le.Uint32(d[q+12:]))), int64(int32(le.Uint32(d[q+16:]))), int64(le.Uint32(d[q+20:]))}
		p.ft = append(p.ft, e)
		c := d[e[6] : e[6]+e[4]]
		var out []byte
		var err error
		switch e[1] {
		case 0:
			out = append([]byte(nil), c...)
		case 1:
			r, e2 := zlib.NewReader(bytes.NewReader(c))
			if e2 != nil {
				return nil, e2
			}
			out, err = io.ReadAll(r)
		case 2:
			out, err = lz4Decode(c, int(e[3]))
		default:
			return nil, fmt.Errorf("compresion %d no soportada", e[1])
		}
		if err != nil {
			return nil, err
		}
		p.Files = append(p.Files, out)
	}
	return p, nil
}

func (p *GFPack) IndexOf(name string) int {
	h := fnv1a64(name)
	for _, f := range p.folders {
		for _, e := range f.files {
			if e[0] == h {
				return int(e[1])
			}
		}
	}
	return -1
}

func (p *GFPack) Get(name string) []byte {
	i := p.IndexOf(name)
	if i < 0 {
		panic("archivo no encontrado en gfpak: " + name)
	}
	return p.Files[i]
}

func (p *GFPack) Set(name string, d []byte) { p.Files[p.IndexOf(name)] = d }

func (p *GFPack) Write() []byte {
	nf, nd := len(p.Files), len(p.folders)
	header := func(ptrFT, ptrHP int, ptrF []int) []byte {
		var b bytes.Buffer
		w := func(v interface{}) { binary.Write(&b, le, v) }
		w(uint64(gfMagic))
		w(p.version)
		w(p.reloc)
		w(int32(nf))
		w(int32(nd))
		w(int64(ptrFT))
		w(int64(ptrHP))
		for _, x := range ptrF {
			w(int64(x))
		}
		hp := b.Len()
		for _, h := range p.hashAbs {
			w(h)
		}
		var pf []int
		for _, f := range p.folders {
			pf = append(pf, b.Len())
			w(f.hash)
			w(int32(len(f.files)))
			w(f.pad)
			for _, e := range f.files {
				w(e[0])
				w(int32(e[1]))
				w(uint32(e[2]))
			}
		}
		pft := b.Len()
		for _, e := range p.ft {
			w(uint16(e[0]))
			w(byte(e[1]))
			w(byte(e[2]))
			w(int32(e[3]))
			w(int32(e[4]))
			w(int32(e[5]))
			w(int32(e[6]))
			w(uint32(e[7]))
		}
		_ = hp
		_ = pft
		return b.Bytes()
	}
	// primera pasada para saber posiciones
	b0 := header(0, 0, make([]int, nd))
	// recomputar punteros
	hp := 0x18 + 16 + 8*nd
	pos := hp + 8*nf
	var pf []int
	for _, f := range p.folders {
		pf = append(pf, pos)
		pos += 16 + 16*len(f.files)
	}
	pft := pos
	out := make([]byte, len(b0))
	for i, d := range p.Files {
		var c []byte
		switch p.ft[i][1] {
		case 0:
			c = d
		case 1:
			var zb bytes.Buffer
			zw, _ := zlib.NewWriterLevel(&zb, 9)
			zw.Write(d)
			zw.Close()
			c = zb.Bytes()
		default:
			c = lz4Encode(d)
		}
		p.ft[i][3] = int64(len(d))
		p.ft[i][4] = int64(len(c))
		p.ft[i][6] = int64(len(out))
		out = append(out, c...)
		for len(out)%16 != 0 {
			out = append(out, 0)
		}
	}
	h := header(pft, hp, pf)
	copy(out, h)
	return out
}

// ---------------- FlatBuffers: lectura ----------------

type FB struct{ B []byte }

func (f FB) u32(o int) int { return int(le.Uint32(f.B[o:])) }
func (f FB) Root() int     { return f.u32(0) }
func (f FB) vt(t int) int  { return t - int(int32(le.Uint32(f.B[t:]))) }
func (f FB) NumFields(t int) int {
	v := f.vt(t)
	return (int(le.Uint16(f.B[v:])) - 4) / 2
}
func (f FB) TableSize(t int) int { return int(le.Uint16(f.B[f.vt(t)+2:])) }
func (f FB) FieldPos(t, i int) int {
	v := f.vt(t)
	vs := int(le.Uint16(f.B[v:]))
	if 4+2*i >= vs {
		return -1
	}
	o := int(le.Uint16(f.B[v+4+2*i:]))
	if o == 0 {
		return -1
	}
	return t + o
}
func (f FB) Table(t, i int) int {
	p := f.FieldPos(t, i)
	if p < 0 {
		return -1
	}
	return p + f.u32(p)
}
func (f FB) Vector(t, i int) []int {
	p := f.FieldPos(t, i)
	if p < 0 {
		return nil
	}
	v := p + f.u32(p)
	n := f.u32(v)
	r := make([]int, n)
	for k := 0; k < n; k++ {
		q := v + 4 + 4*k
		r[k] = q + f.u32(q)
	}
	return r
}

// VecRaw devuelve (posicion del primer elemento, cantidad)
func (f FB) VecRaw(t, i int) (int, int) {
	p := f.FieldPos(t, i)
	if p < 0 {
		return -1, 0
	}
	v := p + f.u32(p)
	return v + 4, f.u32(v)
}
func (f FB) U8(t, i int) uint8 {
	if p := f.FieldPos(t, i); p >= 0 {
		return f.B[p]
	}
	return 0
}
func (f FB) U16(t, i int) uint16 {
	if p := f.FieldPos(t, i); p >= 0 {
		return le.Uint16(f.B[p:])
	}
	return 0
}
func (f FB) U32(t, i int) uint32 {
	if p := f.FieldPos(t, i); p >= 0 {
		return le.Uint32(f.B[p:])
	}
	return 0
}
func (f FB) U64(t, i int) uint64 {
	if p := f.FieldPos(t, i); p >= 0 {
		return le.Uint64(f.B[p:])
	}
	return 0
}
func (f FB) F32(t, i int) float32 {
	if p := f.FieldPos(t, i); p >= 0 {
		return math.Float32frombits(le.Uint32(f.B[p:]))
	}
	return 0
}
func (f FB) Str(t, i int) string {
	p := f.FieldPos(t, i)
	if p < 0 {
		return ""
	}
	s := p + f.u32(p)
	n := f.u32(s)
	return string(f.B[s+4 : s+4+n])
}
func (f FB) SetU8(t, i int, v uint8) bool {
	if p := f.FieldPos(t, i); p >= 0 {
		f.B[p] = v
		return true
	}
	return false
}
func (f FB) SetU16(t, i int, v uint16) bool {
	if p := f.FieldPos(t, i); p >= 0 {
		le.PutUint16(f.B[p:], v)
		return true
	}
	return false
}
func (f FB) SetU32(t, i int, v uint32) bool {
	if p := f.FieldPos(t, i); p >= 0 {
		le.PutUint32(f.B[p:], v)
		return true
	}
	return false
}
func (f FB) SetU64(t, i int, v uint64) bool {
	if p := f.FieldPos(t, i); p >= 0 {
		le.PutUint64(f.B[p:], v)
		return true
	}
	return false
}
func (f FB) Floats(p, n int) []float32 {
	r := make([]float32, n)
	for k := range r {
		r[k] = math.Float32frombits(le.Uint32(f.B[p+4*k:]))
	}
	return r
}

// ---------------- FlatBuffers: escritura ----------------

type fbField struct {
	slot  int
	size  int // 1,2,4,8 escalares; 12/24 structs (alineados a 4); 4 para referencias
	raw   []byte
	child interface{} // *Tbl, *VecT, *VecS, Str
}
type Tbl struct{ fields []fbField }
type VecT struct{ Elems []interface{} } // vector de tablas/strings
type VecS struct {                      // vector de escalares/structs
	Align int
	Raw   []byte
	N     int
}
type Str string

func NewTbl() *Tbl { return &Tbl{} }
func (t *Tbl) U8(slot int, v uint8) *Tbl {
	t.fields = append(t.fields, fbField{slot: slot, size: 1, raw: []byte{v}})
	return t
}
func (t *Tbl) U16(slot int, v uint16) *Tbl {
	b := make([]byte, 2)
	le.PutUint16(b, v)
	t.fields = append(t.fields, fbField{slot: slot, size: 2, raw: b})
	return t
}
func (t *Tbl) U32(slot int, v uint32) *Tbl {
	b := make([]byte, 4)
	le.PutUint32(b, v)
	t.fields = append(t.fields, fbField{slot: slot, size: 4, raw: b})
	return t
}
func (t *Tbl) U64(slot int, v uint64) *Tbl {
	b := make([]byte, 8)
	le.PutUint64(b, v)
	t.fields = append(t.fields, fbField{slot: slot, size: 8, raw: b})
	return t
}
func (t *Tbl) Floats(slot int, v ...float32) *Tbl {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		le.PutUint32(b[4*i:], math.Float32bits(x))
	}
	t.fields = append(t.fields, fbField{slot: slot, size: 4 * len(v), raw: b})
	return t
}
func (t *Tbl) Ref(slot int, c interface{}) *Tbl {
	t.fields = append(t.fields, fbField{slot: slot, size: 4, child: c})
	return t
}
func VecU16(v []uint16) *VecS {
	b := make([]byte, 2*len(v))
	for i, x := range v {
		le.PutUint16(b[2*i:], x)
	}
	return &VecS{Align: 2, Raw: b, N: len(v)}
}
func VecU32(v []uint32) *VecS {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		le.PutUint32(b[4*i:], x)
	}
	return &VecS{Align: 4, Raw: b, N: len(v)}
}
func VecBytes(v []byte) *VecS { return &VecS{Align: 4, Raw: v, N: len(v)} }

type fbWriter struct {
	buf   []byte
	queue []struct {
		at int
		n  interface{}
	}
}

func (w *fbWriter) align(a int, extra int) {
	for (len(w.buf)+extra)%a != 0 {
		w.buf = append(w.buf, 0)
	}
}
func (w *fbWriter) ref(at int, n interface{}) {
	w.queue = append(w.queue, struct {
		at int
		n  interface{}
	}{at, n})
}
func (w *fbWriter) emit(n interface{}) int {
	switch x := n.(type) {
	case *Tbl:
		fs := append([]fbField(nil), x.fields...)
		rank := func(f fbField) int {
			switch {
			case f.size == 8:
				return 0
			case f.size%4 == 0:
				return 1
			case f.size == 2:
				return 2
			}
			return 3
		}
		sort.SliceStable(fs, func(a, b int) bool { return rank(fs[a]) < rank(fs[b]) })
		maxSlot := -1
		has8 := false
		for _, f := range fs {
			if f.slot > maxSlot {
				maxSlot = f.slot
			}
			if f.size == 8 {
				has8 = true
			}
		}
		offs := make([]int, maxSlot+1)
		o := 4
		for _, f := range fs {
			a := 1
			switch {
			case f.size >= 8 && f.size == 8:
				a = 8
			case f.size%4 == 0:
				a = 4
			case f.size == 2:
				a = 2
			}
			for o%a != 0 {
				o++
			}
			offs[f.slot] = o
			o += f.size
		}
		tsize := o
		for tsize%4 != 0 {
			tsize++
		}
		// vtable
		w.align(2, 0)
		vt := len(w.buf)
		w.buf = append(w.buf, make([]byte, 4+2*(maxSlot+1))...)
		le.PutUint16(w.buf[vt:], uint16(4+2*(maxSlot+1)))
		le.PutUint16(w.buf[vt+2:], uint16(tsize))
		for s := 0; s <= maxSlot; s++ {
			le.PutUint16(w.buf[vt+4+2*s:], uint16(offs[s]))
		}
		if has8 {
			w.align(8, 4)
		} else {
			w.align(4, 0)
		}
		ts := len(w.buf)
		w.buf = append(w.buf, make([]byte, tsize)...)
		le.PutUint32(w.buf[ts:], uint32(int32(ts-vt)))
		for _, f := range fs {
			p := ts + offs[f.slot]
			if f.child != nil {
				w.ref(p, f.child)
			} else {
				copy(w.buf[p:], f.raw)
			}
		}
		return ts
	case *VecS:
		a := x.Align
		if a < 4 {
			a = 4
		}
		w.align(a, 4)
		p := len(w.buf)
		w.buf = append(w.buf, 0, 0, 0, 0)
		le.PutUint32(w.buf[p:], uint32(x.N))
		w.buf = append(w.buf, x.Raw...)
		return p
	case *VecT:
		w.align(4, 0)
		p := len(w.buf)
		w.buf = append(w.buf, make([]byte, 4+4*len(x.Elems))...)
		le.PutUint32(w.buf[p:], uint32(len(x.Elems)))
		for i, e := range x.Elems {
			w.ref(p+4+4*i, e)
		}
		return p
	case Str:
		w.align(4, 0)
		p := len(w.buf)
		w.buf = append(w.buf, 0, 0, 0, 0)
		le.PutUint32(w.buf[p:], uint32(len(x)))
		w.buf = append(w.buf, []byte(x)...)
		w.buf = append(w.buf, 0)
		return p
	}
	panic("nodo flatbuffer desconocido")
}

func BuildFB(root *Tbl) []byte {
	w := &fbWriter{buf: make([]byte, 4)}
	w.ref(0, root)
	for len(w.queue) > 0 {
		q := w.queue[0]
		w.queue = w.queue[1:]
		pos := w.emit(q.n)
		le.PutUint32(w.buf[q.at:], uint32(pos-q.at))
	}
	w.align(16, 0)
	return w.buf
}

// AppendFB serializa n al final de buf y hace que la referencia en la posicion at apunte a el.
func AppendFB(buf []byte, at int, n interface{}) []byte {
	w := &fbWriter{buf: append([]byte(nil), buf...)}
	w.align(16, 0)
	w.ref(at, n)
	for len(w.queue) > 0 {
		q := w.queue[0]
		w.queue = w.queue[1:]
		pos := w.emit(q.n)
		le.PutUint32(w.buf[q.at:], uint32(pos-q.at))
	}
	w.align(16, 0)
	return w.buf
}

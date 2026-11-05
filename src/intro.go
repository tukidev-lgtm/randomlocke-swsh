package main

import "os"

// Texto de bienvenida del mod (solo en espanol), en la retransmision de Rose del principio.
var introLines = map[int]string{
	1:  "¡Hola, querido público randomlocker!",
	2:  "Bienvenidos a este mod creado por @Tukidev\npara toda la comunidad randomlocker.",
	3:  "Aquí nada es lo que parece: cada ruta, cada\nentrenador y cada Pokémon serán un misterio.",
	4:  "Solo hay encuentros ocultos en la hierba,\ny el primero de cada zona es el que cuenta.",
	5:  "Los Entrenadores de Galar han estudiado mucho\ny ahora no os lo pondrán nada fácil.",
	6:  "Además, el Repartir Experiencia ya no existe:\ncada nivel habrá que ganárselo con esfuerzo.",
	7:  "Por suerte, las tiendas regalan Caramelos\nRaros. ¡Respetad siempre el límite de nivel!",
	8:  "Quienes se atreven con este desafío son\nconocidos como...",
	9:  "¡Los Randomlockers!",
	10: "Ah, me he dejado llevar por la emoción. Espero\nque disfrutéis el mod. Por cierto, soy Rose.",
}

func textLinesRaw(d []byte) (raw [][]uint16, flags []uint16) {
	n := int(le.Uint16(d[2:]))
	sdo := int(le.Uint32(d[0xC:]))
	key := uint16(0x7C89)
	for i := 0; i < n; i++ {
		off := int(int32(le.Uint32(d[sdo+4+8*i:])))
		ln := int(le.Uint16(d[sdo+8+8*i:]))
		flags = append(flags, le.Uint16(d[sdo+10+8*i:]))
		k := key
		l := make([]uint16, ln)
		for j := 0; j < ln; j++ {
			l[j] = le.Uint16(d[sdo+off+2*j:]) ^ k
			k = k<<3 | k>>13
		}
		raw = append(raw, l)
		key += 0x2983
	}
	return
}

func buildText(header []byte, raw [][]uint16, flags []uint16) []byte {
	n := len(raw)
	sdo := 0x10
	out := append([]byte(nil), header[:sdo]...)
	sec := make([]byte, 4+8*n)
	used := 4 + 8*n
	key := uint16(0x7C89)
	var data []byte
	for i, l := range raw {
		le.PutUint32(sec[4+8*i:], uint32(used))
		le.PutUint16(sec[8+8*i:], uint16(len(l)))
		le.PutUint16(sec[10+8*i:], flags[i])
		k := key
		for _, ch := range l {
			data = append(data, byte(ch^k), byte((ch^k)>>8))
			k = k<<3 | k>>13
		}
		used += 2 * len(l)
		if used%4 == 2 {
			data = append(data, 0, 0)
			used += 2
		}
		key += 0x2983
	}
	sec = append(sec, data...)
	le.PutUint32(sec[0:], uint32(len(sec)))
	out = append(out, sec...)
	le.PutUint16(out[2:], uint16(n))
	le.PutUint32(out[4:], uint32(len(sec)))
	return out
}

func encodeLine(s string) []uint16 {
	var r []uint16
	for _, ch := range s {
		if ch == '\n' {
			r = append(r, 0x0A)
		} else {
			r = append(r, uint16(ch))
		}
	}
	return append(r, 0)
}

func (c *Ctx) introText() {
	rel := "bin/message/Spanish/script/demo_010.dat"
	d, err := os.ReadFile(c.path(rel))
	if err != nil {
		return
	}
	raw, flags := textLinesRaw(d)
	if len(raw) < 13 {
		return
	}
	for i, s := range introLines {
		raw[i] = encodeLine(s)
	}
	c.put(rel, buildText(d, raw, flags))
	c.logf("Texto de bienvenida del mod (espanol) en la retransmision inicial")
}

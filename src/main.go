package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const version = "1.1"

var in = bufio.NewReader(os.Stdin)

func ask(q string) string {
	fmt.Print(q)
	s, _ := in.ReadString('\n')
	return strings.Trim(strings.TrimSpace(s), "\"")
}

// acepta la carpeta del volcado, una de sus subcarpetas, o varias rutas arrastradas a la vez
func splitPaths(s string) []string {
	s = strings.TrimSpace(s)
	if _, err := os.Stat(strings.Trim(s, "\"")); err == nil {
		return []string{strings.Trim(s, "\"")}
	}
	var out []string
	var cur strings.Builder
	inQ := false
	for _, ch := range s {
		switch {
		case ch == '"':
			inQ = !inQ
		case ch == ' ' && !inQ:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(ch)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	// unir trozos de rutas con espacios sin comillas
	var merged []string
	acc := ""
	for _, t := range out {
		if acc == "" {
			acc = t
		} else {
			acc += " " + t
		}
		if _, err := os.Stat(acc); err == nil {
			merged = append(merged, acc)
			acc = ""
		}
	}
	if acc != "" {
		merged = append(merged, out...)
	}
	return merged
}

func findRom(input string) (string, bool) {
	for _, p := range splitPaths(input) {
		if r, ok := findRomOne(p); ok {
			return r, true
		}
	}
	return "", false
}

func findRomOne(p string) (string, bool) {
	p = strings.Trim(p, "\" ")
	for _, cand := range []string{p, filepath.Dir(p)} {
		if st, err := os.Stat(filepath.Join(cand, "romfs", "bin")); err == nil && st.IsDir() {
			if _, err := os.Stat(filepath.Join(cand, "exefs", "main")); err == nil {
				return cand, true
			}
		}
	}
	return "", false
}

var knownBuilds = map[string]byte{"A3B75BCD3311385A": 'k'}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fatal(fmt.Sprint(r))
		}
	}()
	fmt.Println("==============================================")
	fmt.Println("  RANDOMLOCKE - Pokemon Espada / Escudo  v" + version)
	fmt.Println("==============================================")
	fmt.Println("Necesitas el volcado de TU juego actualizado (1.3.2) con las carpetas romfs y exefs.")
	fmt.Println()
	var dump string
	if len(os.Args) > 1 {
		dump = strings.Join(os.Args[1:], " ")
	} else {
		dump = ask("Arrastra aqui la carpeta del volcado (la que tiene romfs y exefs) y pulsa Enter:\n> ")
	}
	root, ok := findRom(dump)
	if !ok {
		fatal("No encuentro las carpetas romfs/bin y exefs/main en: " + dump)
	}
	c := &Ctx{outFiles: map[string][]byte{}}
	c.rom = filepath.Join(root, "romfs")
	exefsMain := filepath.Join(root, "exefs", "main")
	raw, _ := os.ReadFile(exefsMain)
	bid := strings.ToUpper(hex.EncodeToString(raw[0x40:0x48]))
	if g, ok := knownBuilds[bid]; ok {
		c.game = g
	} else {
		for {
			a := ask("No reconozco la version. ¿Es (1) Espada o (2) Escudo? ")
			if a == "1" {
				c.game = 'k'
				break
			}
			if a == "2" {
				c.game = 't'
				break
			}
		}
	}
	juego, tid := "Espada", "0100ABF008968000"
	if c.game == 't' {
		juego, tid = "Escudo", "01008DB008C2C000"
	}
	fmt.Println("Juego:", juego)
	s := ask("Semilla (Enter = aleatoria): ")
	if s == "" {
		c.seed = time.Now().UnixNano() % 1000000000
	} else if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		c.seed = v
	} else {
		var h int64
		for _, ch := range s {
			h = h*31 + int64(ch)
		}
		c.seed = h
	}
	c.rng = rand.New(rand.NewSource(c.seed))
	c.logf("RANDOMLOCKE v%s - Pokemon %s - semilla %d", version, juego, c.seed)
	c.logf("Fecha: %s\n", time.Now().Format("2006-01-02 15:04"))

	step := func(msg string) { fmt.Println(" -", msg) }
	step("Leyendo datos del juego...")
	c.loadData()
	step("Randomizando habilidades, movimientos y tutores...")
	c.randomizeAbilities()
	c.randomizeLearnsets()
	c.randomizeTutors()
	c.tradeEvolutions()
	pc := &poolCache{c: c, m: map[[2]int][]int{}}
	step("Randomizando Pokemon salvajes...")
	dt, wildByLevel := c.randomizeWild(pc)
	pl, err := ReadGFPack(c.read("bin/archive/field/resident/placement.gfpak"))
	if err != nil {
		fatal(err.Error())
	}
	c.disableSymbols(pl)
	step("Randomizando iniciales, regalos, estaticos e intercambios...")
	starterMap := c.randomizeGifts(pc, pl)
	c.randomizeStatics(pc)
	c.randomizeTrades(pc, wildByLevel)
	step("Randomizando objetos del mapa...")
	c.randomizeFieldItems(pl)
	step("Randomizando entrenadores (IA mejorada, equipos progresivos)...")
	c.randomizeTrainers(pc, starterMap)
	step("Creando hierba en cuevas y zonas sin hierba...")
	c.logf("")
	c.caveGrass(pl)
	c.put("bin/archive/field/resident/placement.gfpak", pl.Write())
	c.put("bin/archive/field/resident/data_table.gfpak", dt.Write())
	c.put("bin/pml/personal/personal_total.bin", c.personal)
	c.put("bin/pml/waza_oboe/wazaoboe_total.bin", c.learn)
	step("Tienda y Repartir Experiencia...")
	c.shopCandies()
	c.introText()
	ipsName, ips := c.expSharePatch()

	// ---- escribir el mod ----
	exeDir, _ := filepath.Abs(filepath.Dir(root))
	modName := fmt.Sprintf("Randomlocke_%d", c.seed)
	outDir := filepath.Join(exeDir, modName)
	os.RemoveAll(outDir)
	for rel, d := range c.outFiles {
		p := filepath.Join(outDir, "romfs", filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0755)
		if err := os.WriteFile(p, d, 0644); err != nil {
			fatal(err.Error())
		}
	}
	if ips != nil {
		os.MkdirAll(filepath.Join(outDir, "exefs"), 0755)
		os.WriteFile(filepath.Join(outDir, "exefs", ipsName+".ips"), ips, 0644)
	}
	os.WriteFile(filepath.Join(exeDir, modName+"_log.txt"), []byte(strings.ReplaceAll(c.log.String(), "\n", "\r\n")), 0644)
	fmt.Println()
	fmt.Println("Mod creado en:")
	fmt.Println("  ", outDir)
	fmt.Println("Registro con spoilers (que salio en cada sitio):")
	fmt.Println("  ", filepath.Join(exeDir, modName+"_log.txt"))

	fmt.Println()
	fmt.Println("COMO INSTALARLO:")
	fmt.Printf("  Emulador (Eden, yuzu, Citron, Sudachi, Ryujinx...): abre la carpeta de mods del juego\n")
	fmt.Printf("  (clic derecho en el juego > abrir carpeta de mods) y copia dentro la carpeta %s.\n", modName)
	fmt.Printf("  Switch con Atmosphere: copia el CONTENIDO de %s (romfs y exefs) en\n", modName)
	fmt.Printf("  sd:/atmosphere/contents/%s/\n", tid)
	fmt.Println("  Desactiva cualquier otro mod de este juego y empieza una partida nueva.")
	fmt.Println("\n¡Listo! Pulsa Enter para salir.")
	if true {
		in.ReadString('\n')
	}
}

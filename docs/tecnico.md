# Notas técnicas

Lo que se descubrió durante el desarrollo, por si sirve a quien quiera hacer mods de Espada/Escudo.
Todas las rutas son relativas a `romfs/`.

## Contenedores y formatos

- **GFPack** (`.gfpak`): contenedor con archivos identificados por hash FNV-1a de 64 bits del nombre.
  Compresión por archivo: 0 = sin comprimir, 1 = zlib, 2 = LZ4 (bloque). Implementado en `src/core.go`.
- Casi todo lo demás son **FlatBuffers** sin identificador. `src/core.go` incluye un lector y un
  escritor genérico. Las referencias (uoffset) siempre apuntan hacia delante; para añadir datos a un
  archivo existente se escriben al final y se redirige la referencia (`AppendFB`).

## Pokémon visibles (Symbol) y ocultos (Hidden)

- Las tablas están en `bin/archive/field/resident/data_table.gfpak`:
  `encount_k.bin` / `encount_t.bin` (ocultos, Espada/Escudo) y `encount_symbol_k.bin` / `encount_symbol_t.bin`
  (visibles). Cada tabla tiene 11 subtablas: 9 climas, árboles y pesca.
- **Una tabla Symbol no puede estar vacía ni tener todo a 0 %**: el juego se cuelga al entrar en la zona.
- Los Pokémon los crean **puntos de aparición** en `placement.gfpak` (`a_xxxx.bin` → zona → campo 20 `Symbols`).
  Cada punto genera **tanto visibles como ocultos**. Tiene 4 bloques de ajustes (`Field_02..05`):
  - **`Field_03.Field_00 = 0` apaga los visibles y mantiene los ocultos.** Es el interruptor de este mod.
  - `Field_02` no tiene efecto visible al apagarlo; `Field_05` parece el radio de aparición.
- Las zonas sin puntos de aparición no tienen encuentros aunque tengan tabla.

## Hierba alta

- **Los encuentros ocultos dependen del material del suelo** de la colisión: `col_kusa` (hierba),
  `col_karekusa` (hierba seca) y `col_yukikusa` (hierba nevada) generan "!". También deciden el sonido
  de los pasos y las partículas, así que no se pueden separar.
- Las colisiones de cada zona están en `bin/archive/field/area/other/a_xxxx.gfpak` (FlatBuffer: materiales,
  bloques con caja, mallas con submallas por material e índices `u16`, y vértices). El mod **recorta los
  triángulos del suelo** por los bordes de cada parche (Sutherland-Hodgman) y mueve los de dentro a
  `col_kusa`; el área total del suelo no cambia.
- Lo que se ve son **instancias** en `bin/field/model/a_xxxx/a_xxxx_coord_encountweedNNN.gfbmdli`
  (FlatBuffer: mallas → grupos con modelo `instance_obj_encountweedXX` → lista de escala/rotación/posición).
  Para que una zona cargue un archivo nuevo hay que añadir una entrada al descriptor de terreno de esa zona,
  dentro de `bin/archive/field/resident/terrain.gfpak`.
- Algunas ciudades (Circhester, Spikemuth) tienen un **suelo de relleno** enorme bajo la ciudad real:
  hay que comprobar la altura de los NPCs para colocar bien la hierba.

## Repartir Experiencia

En el ejecutable (`exefs/main`), la función que calcula la experiencia hace:

```
ldrb w9, [x0, #0x20]   ; ¿ha combatido?
...
cbnz w9, calcular
ldrb w8, [x20, #0x21]  ; ¿Repartir Experiencia?
cbz  w8, devolver_0    ; <- se cambia por "b devolver_0"
```

En Espada 1.3.2 está en `0x8A5A34` (texto). El programa la localiza **por patrón**, así que debería
funcionar también en Escudo. Ver `patches/`.

## Entrenadores

- `bin/trainer/trainer_data/*.bin`: byte 3 = número de Pokémon, `0x0C` = IA (bits: 0 Basic, 1 Strong,
  2 Expert, 3 Doubles, 5 UseItem, 6 PokeChange).
- `bin/trainer/trainer_poke/*.bin`: 0x20 bytes por Pokémon (nivel `0x0A`, especie `0x0C`, forma `0x0E`,
  objeto `0x10`, movimientos `0x12`).

## Textos

`bin/message/<idioma>/script/*.dat` y `common/*.dat`: formato de texto de Gen 8 (cifrado XOR por línea).
La retransmisión inicial es `script/demo_010.dat`. No se pueden añadir líneas sin tocar el script que las llama.

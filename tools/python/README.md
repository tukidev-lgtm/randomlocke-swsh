# Scripts de investigación

Scripts en Python usados para investigar los formatos antes de escribir el programa en Go.
No hacen falta para usar el randomizador. Requieren `pip install lz4 flatbuffers`.

| Script | Para qué sirve |
|---|---|
| `gfpak.py` | Leer y escribir contenedores `.gfpak` y navegar FlatBuffers |
| `fbdump.py` | Volcar la estructura de un FlatBuffer desconocido |
| `encfb.py` | Leer y reconstruir las tablas de encuentros |
| `colfb.py`, `colmesh.py`, `colrename.py` | Colisiones: leer, reconstruir y renombrar materiales |
| `floor.py`, `patches.py` | Consultar la altura del suelo y recortar triángulos |
| `cavegrass.py`, `weedread.py` | Crear y leer archivos de matojos (`.gfbmdli`) y descriptores de terreno |
| `textdump.py` | Volcar textos del juego mostrando los códigos de control |
| `trainers_info.py`, `trainers_mod.py` | Listar entrenadores y aplicar IA/equipos (versión Python) |

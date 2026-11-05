#!/bin/sh
# Compila el randomizador (Windows y Linux). Necesitas Go 1.22 o superior.
cd "$(dirname "$0")"
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o ../dist/Randomlocke.exe . &&
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../dist/randomlocke_linux . &&
echo "Listo: dist/Randomlocke.exe y dist/randomlocke_linux"

// Package webfs embebe el frontend estático (web/) dentro del binario
// compilado, en vez de leerlo del disco en runtime con http.Dir. Esto
// evita depender de cuál sea el directorio de trabajo del proceso — en
// Railway y en local el cwd es la raíz del repo, pero en Vercel el
// proceso puede arrancar desde otro directorio, y http.Dir("web") ahí no
// encontraba los archivos. Con //go:embed, el contenido de web/ queda
// adentro del ejecutable sin importar desde dónde se lo corra.
package webfs

import "embed"

//go:embed web
var FS embed.FS

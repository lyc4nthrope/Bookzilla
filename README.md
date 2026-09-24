# Bookzilla

Aplicación web en **Go** (net/http + html/template) con catálogo de libros: alta,
edición y baja desde la interfaz web, con portadas subidas por el usuario.
Plantillas y estáticos van embebidos en el binario (`go:embed`); los datos y las
imágenes viven en carpetas externas y Bootstrap está vendorizado (funciona sin internet).

## Requisitos

- Go 1.27+

## Correr la app

```powershell
go run .
```

Escucha en el puerto `8080` por defecto. Puerto configurable por variable de entorno:

```powershell
$env:PORT = "3000"; go run .
```

Abrí <http://localhost:8080/>.

Al primer arranque se autogestionan dos carpetas en el **directorio de trabajo actual**
(la carpeta desde donde se ejecuta el binario):

- `data/` — `books.json`, el catálogo editable.
- `images/` — portadas (las 12 seed + las subidas).

Si no existen se crean (`os.MkdirAll`). El seed viene **embebido en el binario**
(carpeta `seed/`, que es un espejo de `data/` + `images/`) y solo se copia lo que
**no existe todavía**: nunca pisa un archivo que el usuario editó. Las portadas seed
se regeneran únicamente si faltan en `images/`.

## Cómo agregar / editar / eliminar un libro

**Desde la interfaz (recomendado):**

1. **Agregar**: botón *Agregar libro* → modal con título, autor, año y portada
   (JPEG, PNG, WebP, GIF o SVG, máx. 5 MB, validados en el servidor por contenido
   real del archivo, no por la extensión). El nombre del archivo lo genera el servidor.
2. **Editar**: botón *Editar* en cada tarjeta → mismo modal precargado. Si no se elige
   una imagen nueva, se conserva la portada actual.
3. **Eliminar**: botón *Eliminar* → pide confirmación y borra el libro; la portada se
   borra solo si ningún otro libro la usa.

Cada operación usa PRG (Post/Redirect/Get) y muestra un alert con el resultado o el
motivo del error.

**A mano (alternativa):** editar `data/books.json` con cualquier editor y agregar un
objeto al array:

```json
{
  "id": "titulo-del-libro",
  "title": "Título del libro",
  "author": "Autor",
  "year": 2020,
  "cover": "images/cover-13.svg"
}
```

`id` debe ser único (string); si falta, el servidor lo genera como slug del título la
primera vez que carga el archivo. La portada es una imagen dentro de `images/` y se
sirve bajo `/images/`. No hace falta recompilar: el JSON se lee en cada request.
Si el archivo es inválido, el servidor **no arranca** y deja el motivo en el log.

## Backup

Copiar las dos carpetas de datos, que viajan separadas del binario:

```powershell
Copy-Item data, images -Recurse -Destination D:\backup-bookzilla\
```

- `data/books.json` es el catálogo (trackeado en git, se versiona con el código).
- `images/` es runtime (ignorada por git): portadas subidas + portadas seed.
  Si se pierde, las portadas seed se regeneran solas en el próximo arranque; las
  subidas **no** se recuperan, por eso hay que incluirlas en el backup.

## Estructura

```
main.go                 # servidor, rutas y embed de templates/, static/ y seed/
store.go                # modelo, lectura/escritura del JSON y seed a disco
handlers.go             # alta/edición/baja, validación y subida de imágenes
seed/                   # semilla embebida (espejo de data/ + images/)
  data/books.json       # catálogo inicial: 12 libros
  images/cover-01..12.svg
data/books.json         # catálogo runtime (editable, se trackea en git)
images/                 # portadas runtime (creada al arrancar, ignorada por git)
templates/index.html    # plantilla HTML5 (header y footer con datos del grupo)
static/vendor/          # Bootstrap 5.3.3 local (css + bundle js)
static/css/styles.css   # estilos propios
static/js/main.js       # búsqueda, modales, PRG y confirmación (vanilla JS)
```

El nombre del host se muestra en el header y el footer, obtenido con `os.Hostname()`.
Los datos del grupo (materia, integrantes, comisión, ciclo) son placeholders en
`templates/index.html` que el alumno debe completar.

## Seguridad

La app **no tiene autenticación**: cualquiera que alcance el puerto puede agregar,
editar o borrar libros y subir imágenes. Es válida para uso local/académico; si se
expone públicamente hay que ponerla detrás de un proxy con autenticación (o agregar
login propio), limitar el tamaño y los tipos de subida (ya hay límites: 5 MB y
solo tipos de imagen) y, idealmente, correrla con un usuario sin permisos de escritura
sensible. No exponerla sin esos controles.

## Despliegue

**Binario único** (templates, estáticos y seed embebidos):

```powershell
go build -o bookzilla.exe .
.\bookzilla.exe          # PORT=8080 por defecto
```

Las carpetas `data/` e `images/` se crean al arrancar junto al binario (en el CWD);
no hace falta copiar nada más.

**Docker**:

```powershell
docker build -t bookzilla .
docker run -p 8080:8080 -e PORT=8080 -v bookzilla-data:/app/data -v bookzilla-images:/app/images bookzilla
```

Los volúmenes conservan el catálogo y las portadas entre reinicios (si no se montan,
se regeneran desde el seed embebido).

**Detrás de un load balancer**: correr una o más instancias apuntando al mismo volumen
`data/` (o una por instancia), health check en `GET /`, y el LB reenvía el tráfico a
`localhost:<PORT>`. El hostname mostrado en la página es el de cada instancia
(`os.Hostname()`), útil para identificar a qué nodo respondió. Ojo: cada instancia
debe compartir **tanto** `data/` **como** `images/`, o las portadas subidas no se ven
en las demás.

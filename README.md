# Bookzilla

Catálogo web de libros en **Go** (`net/http` + `html/template`) para la actividad
*"Desarrollo de una aplicación web en Golang: Catálogo de libros"* de
**Computación en la Nube** (Universidad del Quindío, 2026-2).

**Grupo:** Cristhian Eduardo Osorio Restrepo · Daniel Stiven Perez Cordoba · William Carmona Diaz

## Requisitos del enunciado y dónde se cumplen

| Requisito | Implementación |
|---|---|
| Catálogo inicial de 10 a 15 libros | 12 libros en `seed/data/books.json` |
| Cada libro: portada, título, autor, año | struct `Book` en `store.go` |
| Datos en JSON leídos por la app | `data/books.json`, releído en cada solicitud (`loadBooks`) |
| Agregar libros con un editor de texto | editar `data/books.json` y recargar; no hace falta reiniciar |
| HTML5, CSS, JavaScript, Bootstrap | `templates/index.html`, `static/css`, `static/js`, Bootstrap 5.3.3 en `static/vendor` |
| Encabezado y banner inferior con los datos del grupo | `<header>` y `<footer>` de la plantilla; los datos salen de `group` en `main.go` |
| Adaptable a distintos tamaños de pantalla | grid de Bootstrap `col-12 col-sm-6 col-md-4 col-xl-3` |
| Portadas como recursos independientes en una carpeta, con su ruta en los datos | carpeta `images/`, campo `"cover": "images/cover-01.svg"`, servida en `/images/` |
| Hostname obtenido del sistema operativo | `os.Hostname()` en `main.go`, visible en encabezado y pie |
| Desplegable en VM, contenedor, balanceador y nube | binario único, puerto por `PORT`, servicio systemd y Dockerfile (ver "Despliegue") |

Extra (no pedido): alta, edición y baja de libros desde la interfaz con subida de portada.

## Ejecutar en local

Requisito: Go 1.27+.

```bash
git clone https://github.com/lyc4nthrope/Bookzilla.git
cd Bookzilla
go run .                 # http://localhost:8080
PORT=3000 go run .       # otro puerto (PowerShell: $env:PORT = "3000"; go run .)
```

En el primer arranque la app crea en el directorio de trabajo:

- `data/books.json`: el catálogo editable.
- `images/`: las portadas (las 12 de la semilla más las que se suban).

Se copian desde `seed/`, que va embebida en el binario, y **solo si faltan**: nunca se
sobrescribe un archivo editado. Ninguna de las dos carpetas se versiona en git.

## Agregar un libro con un editor de texto

1. Copiar la imagen a `images/` (por ejemplo, `images/mi-libro.jpg`).
2. Agregar un objeto al arreglo de `data/books.json`:

```json
{
  "id": "mi-libro",
  "title": "Mi libro",
  "author": "Autor",
  "year": 2020,
  "cover": "images/mi-libro.jpg"
}
```

3. Recargar la página.

El `id` debe ser único. Si se omite, la app lo genera a partir del título. Si el JSON queda
mal escrito, la página responde con un error 500 y el log indica el motivo. Además, en
ese estado el servidor no vuelve a arrancar hasta que se corrija el archivo.

## Estructura

```
main.go                 # arranque, datos del grupo, rutas y hostname
store.go                # modelo Book, lectura/escritura atómica del JSON, semilla
handlers.go             # alta/edición/baja, validación y subida de portadas
templates/index.html    # plantilla HTML5 con encabezado y banner inferior
static/                 # CSS propio, JS y Bootstrap local (funciona sin Internet)
seed/                   # catálogo inicial y portadas embebidas en el binario
deploy/bookzilla.service# unidad systemd para la VM
Dockerfile              # imagen de contenedor
```

## Despliegue en una máquina virtual (VirtualBox)

Este despliegue usa lo trabajado en clase: una VM Debian creada con `VBoxManage` y
adaptador puente (Taller #1), un disco de datos montado en `/mnt/datos` (Laboratorio #5 y
Nota parcial 1) y snapshots (Laboratorio #3).

**1. VM.** Debian 13 de 64 bits con adaptador de red en **modo puente**, para que la
aplicación sea accesible desde otros equipos de la red, y Guest Additions instaladas.

**2. Disco de datos.** Montado en `/mnt/datos` y registrado en `/etc/fstab` por UUID,
como en la Nota parcial 1. El catálogo y las portadas quedan en ese disco, separados del
sistema operativo: el disco se puede desconectar, respaldar o conectar a otra VM sin
perder los libros.

**3. Snapshot previo**, para poder volver atrás si algo falla:

```bash
VBoxManage snapshot "<vm>" take "antes-de-bookzilla" --description "VM limpia antes de instalar Bookzilla"
```

**4. Compilar en el host y copiar el binario a la VM.** Go compila para Linux desde
cualquier sistema operativo y la VM no necesita tener Go instalado:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bookzilla .
VBoxManage guestcontrol "<vm>" copyto --username <usuario> --password <clave> \
  --target-directory /tmp bookzilla deploy/bookzilla.service
```

**5. Instalar como servicio** (dentro de la VM):

```bash
sudo useradd --system --no-create-home bookzilla
sudo install -D -m 755 /tmp/bookzilla /opt/bookzilla/bookzilla
sudo install -d -o bookzilla -g bookzilla /mnt/datos/bookzilla
sudo cp /tmp/bookzilla.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now bookzilla
systemctl status bookzilla
```

systemd arranca la app en cada inicio de la VM, espera a que `/mnt/datos` esté montado
(`RequiresMountsFor`), la reinicia si falla y la ejecuta con un usuario sin privilegios.

**6. Probar desde el host.** Se obtiene la IP de la VM con Guest Additions y se abre en
el navegador:

```bash
VBoxManage guestproperty get "<vm>" "/VirtualBox/GuestInfo/Net/0/V4/IP"
# abrir http://<ip>:8080 → el encabezado muestra "Servidor: <hostname de la VM>"
```

**Varias VMs detrás de un balanceador.** Se hace un clon completo con nuevas direcciones
MAC (Laboratorio #4) y se cambia su hostname (`sudo hostnamectl set-hostname bookzilla-2`).
Cada VM muestra su propio nombre, así se ve qué nodo atendió cada solicitud. Cada instancia
lee su propio `data/`: un libro agregado en una VM no aparece en las otras.

## Despliegue en contenedor

```bash
docker build -t bookzilla .
docker run -p 8080:8080 -v bookzilla-data:/app/data -v bookzilla-images:/app/images bookzilla
```

En un contenedor el hostname es el ID del contenedor.

## Seguridad

- La app **no tiene autenticación**: cualquiera que llegue al puerto puede modificar el
  catálogo. Es adecuada para uso académico o en red local.
- Solo se aceptan portadas JPEG, PNG, WebP o GIF de hasta 5 MB. El tipo se valida por el
  contenido real del archivo, y el nombre en disco lo genera el servidor.
- No se aceptan SVG subidos porque pueden contener JavaScript. Las portadas SVG de la
  semilla son del proyecto y son seguras.

package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Límites de validación: el servidor no confía en lo que valida el navegador.
const (
	maxImageBytes = 5 << 20
	maxFormBytes  = maxImageBytes + 64<<10
	maxFieldRunes = 200
	minYear       = 1000
	maxYear       = 2100
)

// imageExtensions lista los formatos de portada que se aceptan al subir. SVG
// queda fuera a propósito: es XML y puede traer <script>; servido desde este
// mismo dominio permitiría XSS. Las portadas SVG de la semilla son propias y
// confiables, por eso siguen funcionando.
var imageExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// formError es un error de validación que se muestra al usuario: code elige
// el estilo del aviso en main.js y motivo es el texto legible.
type formError struct {
	code   string
	motivo string
}

func (e *formError) Error() string { return e.motivo }

// bookForm es el formulario ya validado: el libro y, si se subió, la portada.
type bookForm struct {
	Book     Book
	Image    []byte
	ImageExt string
}

// Los tres handlers siguen el mismo esquema: validar el formulario, tomar el
// mutex, releer el JSON, modificar la lista, guardarla y redirigir (PRG).

// handleCreateBook agrega un libro nuevo con un id derivado del título.
func handleCreateBook(w http.ResponseWriter, r *http.Request) {
	form, ferr := parseBookForm(w, r)
	if ferr != nil {
		redirectMsg(w, r, ferr.code, ferr.motivo)
		return
	}

	booksMu.Lock()
	defer booksMu.Unlock()
	books, err := loadBooks(booksFile)
	if err != nil {
		log.Printf("error cargando %s: %v", booksFile, err)
		redirectMsg(w, r, "err-guardado", "No se pudo leer el catalogo")
		return
	}

	book := form.Book
	book.ID = uniqueID(book.Title, idsOf(books))
	if len(form.Image) > 0 {
		name, err := saveImage(form.Image, form.ImageExt)
		if err != nil {
			log.Printf("error guardando la portada: %v", err)
			redirectMsg(w, r, "err-imagen", "No se pudo guardar la imagen en disco")
			return
		}
		book.Cover = imagesDir + "/" + name
	}
	books = append(books, book)
	if err := saveBooks(booksFile, books); err != nil {
		log.Printf("error guardando %s: %v", booksFile, err)
		redirectMsg(w, r, "err-guardado", "No se pudo escribir el catalogo en disco")
		return
	}
	redirectMsg(w, r, "ok-agregado", "")
}

// handleEditBook reemplaza los datos de un libro; si no llega portada nueva,
// conserva la anterior.
func handleEditBook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	form, ferr := parseBookForm(w, r)
	if ferr != nil {
		redirectMsg(w, r, ferr.code, ferr.motivo)
		return
	}

	booksMu.Lock()
	defer booksMu.Unlock()
	books, err := loadBooks(booksFile)
	if err != nil {
		log.Printf("error cargando %s: %v", booksFile, err)
		redirectMsg(w, r, "err-guardado", "No se pudo leer el catalogo")
		return
	}
	position := indexOfBook(books, id)
	if position < 0 {
		redirectMsg(w, r, "err-no-encontrado", "No existe un libro con ese identificador")
		return
	}

	book := form.Book
	book.ID = id
	previousCover := books[position].Cover
	if len(form.Image) > 0 {
		name, err := saveImage(form.Image, form.ImageExt)
		if err != nil {
			log.Printf("error guardando la portada: %v", err)
			redirectMsg(w, r, "err-imagen", "No se pudo guardar la imagen en disco")
			return
		}
		book.Cover = imagesDir + "/" + name
	} else {
		book.Cover = previousCover
	}
	books[position] = book
	if err := saveBooks(booksFile, books); err != nil {
		log.Printf("error guardando %s: %v", booksFile, err)
		redirectMsg(w, r, "err-guardado", "No se pudo escribir el catalogo en disco")
		return
	}
	if len(form.Image) > 0 {
		removeCoverIfUnused(previousCover, books)
	}
	redirectMsg(w, r, "ok-editado", "")
}

// handleDeleteBook quita un libro y borra su portada si ningún otro la usa.
func handleDeleteBook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	booksMu.Lock()
	defer booksMu.Unlock()
	books, err := loadBooks(booksFile)
	if err != nil {
		log.Printf("error cargando %s: %v", booksFile, err)
		redirectMsg(w, r, "err-guardado", "No se pudo leer el catalogo")
		return
	}
	position := indexOfBook(books, id)
	if position < 0 {
		redirectMsg(w, r, "err-no-encontrado", "No existe un libro con ese identificador")
		return
	}

	cover := books[position].Cover
	books = append(books[:position], books[position+1:]...)
	if err := saveBooks(booksFile, books); err != nil {
		log.Printf("error guardando %s: %v", booksFile, err)
		redirectMsg(w, r, "err-guardado", "No se pudo escribir el catalogo en disco")
		return
	}
	removeCoverIfUnused(cover, books)
	redirectMsg(w, r, "ok-eliminado", "")
}

// redirectMsg implementa Post/Redirect/Get: tras un POST redirige a "/" con el
// resultado en la URL, así recargar la página no reenvía el formulario.
func redirectMsg(w http.ResponseWriter, r *http.Request, msg, motivo string) {
	query := url.Values{"msg": {msg}}
	if motivo != "" {
		query.Set("motivo", motivo)
	}
	http.Redirect(w, r, "/?"+query.Encode(), http.StatusFound)
}

// parseBookForm lee y valida título, autor, año y portada. Acepta multipart
// (con archivo) y formularios simples; MaxBytesReader corta cuerpos gigantes.
func parseBookForm(w http.ResponseWriter, r *http.Request) (*bookForm, *formError) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseMultipartForm(maxFormBytes); err != nil {
		if !errors.Is(err, http.ErrNotMultipart) {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				return nil, &formError{"err-imagen", "El formulario supera el maximo de 5 MB"}
			}
			return nil, &formError{"err-validacion", "No se pudo leer el formulario"}
		}
		if err := r.ParseForm(); err != nil {
			return nil, &formError{"err-validacion", "No se pudo leer el formulario"}
		}
	}

	title, ferr := textField(r, "title", "título")
	if ferr != nil {
		return nil, ferr
	}
	author, ferr := textField(r, "author", "autor")
	if ferr != nil {
		return nil, ferr
	}
	year, ferr := yearField(r)
	if ferr != nil {
		return nil, ferr
	}

	image, ext, ferr := readUpload(r)
	if ferr != nil {
		return nil, ferr
	}
	return &bookForm{
		Book:     Book{Title: title, Author: author, Year: year},
		Image:    image,
		ImageExt: ext,
	}, nil
}

// textField exige un texto no vacío de hasta maxFieldRunes caracteres.
func textField(r *http.Request, name, label string) (string, *formError) {
	value := strings.TrimSpace(r.FormValue(name))
	if value == "" {
		return "", &formError{"err-validacion", "El " + label + " es obligatorio"}
	}
	if utf8.RuneCountInString(value) > maxFieldRunes {
		return "", &formError{"err-validacion", "El " + label + " no puede superar 200 caracteres"}
	}
	return value, nil
}

// yearField exige un año entero dentro de [minYear, maxYear].
func yearField(r *http.Request) (int, *formError) {
	value := strings.TrimSpace(r.FormValue("year"))
	year, err := strconv.Atoi(value)
	if err != nil {
		return 0, &formError{"err-validacion", "El año debe ser un número entero"}
	}
	if year < minYear || year > maxYear {
		return 0, &formError{"err-validacion", fmt.Sprintf("El año debe estar entre %d y %d", minYear, maxYear)}
	}
	return year, nil
}

// readUpload devuelve la portada subida y su extensión, o nil si no se envió.
func readUpload(r *http.Request) ([]byte, string, *formError) {
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil, "", nil
	}
	headers := r.MultipartForm.File["cover"]
	if len(headers) == 0 || headers[0].Filename == "" {
		return nil, "", nil
	}
	file, err := headers[0].Open()
	if err != nil {
		return nil, "", &formError{"err-imagen", "No se pudo leer el archivo de imagen"}
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		return nil, "", &formError{"err-imagen", "No se pudo leer el archivo de imagen"}
	}
	if len(data) == 0 {
		return nil, "", &formError{"err-imagen", "El archivo de imagen está vacío"}
	}
	if len(data) > maxImageBytes {
		return nil, "", &formError{"err-imagen", "La imagen no puede superar 5 MB"}
	}
	// El formato se decide por el contenido real (firma de los primeros bytes),
	// no por la extensión ni por el Content-Type que declara el navegador.
	ext, ok := imageExtensions[http.DetectContentType(data)]
	if !ok {
		return nil, "", &formError{"err-imagen", "Formato de imagen no admitido: usa JPEG, PNG, WebP o GIF"}
	}
	return data, ext, nil
}

// saveImage guarda la portada con un nombre aleatorio generado por el
// servidor, así el nombre que envía el usuario nunca llega al sistema de archivos.
func saveImage(data []byte, ext string) (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	name := hex.EncodeToString(buf[:]) + ext
	dest := filepath.Join(imagesDir, name)
	if !withinImages(dest) {
		return "", errors.New("ruta de portada fuera de " + imagesDir)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", err
	}
	return name, nil
}

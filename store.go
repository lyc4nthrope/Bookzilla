package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

// Rutas relativas al directorio de trabajo: en la VM el servicio systemd fija
// ese directorio en el disco de datos (ver deploy/bookzilla.service).
const (
	booksFile = "data/books.json"
	imagesDir = "images"
	seedDir   = "seed"
)

// booksMu serializa lectura-modificación-escritura del JSON para que dos
// solicitudes simultáneas no se pisen los cambios.
var booksMu sync.Mutex

// Book es un libro del catálogo. Cover es la ruta de la portada dentro de
// images/, tal como pide el enunciado.
type Book struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Year   int    `json:"year"`
	Cover  string `json:"cover"`
}

// rawBook tolera ids faltantes o numéricos en un JSON editado a mano.
type rawBook struct {
	ID     json.RawMessage `json:"id"`
	Title  string          `json:"title"`
	Author string          `json:"author"`
	Year   int             `json:"year"`
	Cover  string          `json:"cover"`
}

// id normaliza el id a string; devuelve "" si falta o no es válido.
func (r rawBook) id() string {
	raw := strings.TrimSpace(string(r.ID))
	if raw == "" || raw == "null" {
		return ""
	}
	if strings.HasPrefix(raw, `"`) {
		var s string
		if err := json.Unmarshal(r.ID, &s); err != nil {
			return ""
		}
		return s
	}
	if _, err := strconv.Atoi(raw); err != nil {
		return ""
	}
	return raw
}

// loadBooks debe llamarse con booksMu tomado; si el JSON venia sin ids,
// los completa y persiste la migracion una sola vez.
func loadBooks(filename string) ([]Book, error) {
	raw, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var rows []rawBook
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	books := make([]Book, 0, len(rows))
	taken := make(map[string]bool, len(rows))
	migrated := false
	for _, row := range rows {
		book := Book{ID: row.id(), Title: row.Title, Author: row.Author, Year: row.Year, Cover: row.Cover}
		if book.ID == "" {
			book.ID = uniqueID(book.Title, taken)
			migrated = true
		}
		taken[book.ID] = true
		books = append(books, book)
	}
	if migrated {
		if err := saveBooks(filename, books); err != nil {
			return nil, fmt.Errorf("persistiendo la migracion de ids: %w", err)
		}
	}
	return books, nil
}

// saveBooks escribe a un archivo temporal y luego lo renombra: el rename es
// atómico, así un corte a mitad de escritura nunca deja el JSON corrupto.
func saveBooks(filename string, books []Book) error {
	data, err := json.MarshalIndent(books, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := filename + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, filename); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// ensureSeed crea data/ e images/ y copia la semilla embebida, pero solo los
// archivos que faltan: nunca pisa datos que el usuario ya editó.
func ensureSeed() error {
	for _, dir := range []string{filepath.Dir(booksFile), imagesDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return fs.WalkDir(seedFS, seedDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, seedDir+"/")
		if rel == path {
			return nil
		}
		dest := filepath.FromSlash(rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		data, err := seedFS.ReadFile(path)
		if err != nil {
			return err
		}
		return writeIfMissing(dest, data)
	})
}

// writeIfMissing usa O_EXCL: crea el archivo solo si no existe.
func writeIfMissing(dest string, data []byte) error {
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// removeCoverIfUnused borra una portada de images/ si ya ningún libro la usa.
func removeCoverIfUnused(cover string, books []Book) {
	if cover == "" || !withinImages(cover) {
		return
	}
	for _, b := range books {
		if b.Cover == cover {
			return
		}
	}
	if err := os.Remove(filepath.FromSlash(cover)); err != nil && !os.IsNotExist(err) {
		log.Printf("no se pudo borrar la portada %s: %v", cover, err)
	}
}

// withinImages evita que una ruta del JSON (p. ej. "../main.go") apunte
// fuera de images/ antes de borrar un archivo.
func withinImages(path string) bool {
	rel, err := filepath.Rel(imagesDir, filepath.Clean(filepath.FromSlash(path)))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func idsOf(books []Book) map[string]bool {
	ids := make(map[string]bool, len(books))
	for _, b := range books {
		ids[b.ID] = true
	}
	return ids
}

func indexOfBook(books []Book, id string) int {
	for i, b := range books {
		if b.ID == id {
			return i
		}
	}
	return -1
}

// uniqueID genera un id legible a partir del título ("cien-anos-de-soledad")
// y agrega -2, -3... si ya existe.
func uniqueID(title string, taken map[string]bool) string {
	base := slugify(title)
	if base == "" {
		base = "libro"
	}
	id := base
	for i := 2; taken[id]; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}
	return id
}

var asciiFallback = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n', 'ß': 's',
}

// slugify pasa el título a minúsculas sin tildes y separa las palabras con guiones.
func slugify(title string) string {
	var b strings.Builder
	pendingDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		if base, ok := asciiFallback[r]; ok {
			r = base
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			pendingDash = false
			continue
		}
		if b.Len() > 0 && !pendingDash {
			b.WriteByte('-')
			pendingDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

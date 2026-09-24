package main

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed templates
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

//go:embed seed
var seedFS embed.FS

type PageData struct {
	Hostname string
	Books    []Book
	Count    int
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("no se pudo obtener el hostname del sistema: %v", err)
		hostname = "desconocido"
	}

	if err := ensureSeed(); err != nil {
		log.Fatalf("error inicializando archivos de datos: %v", err)
	}

	booksMu.Lock()
	books, err := loadBooks(booksFile)
	booksMu.Unlock()
	if err != nil {
		log.Fatalf("el catalogo no es valido, revisa %s: %v", booksFile, err)
	}
	log.Printf("catalogo cargado desde %s: %d libros", booksFile, len(books))

	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		log.Fatalf("error parseando plantillas: %v", err)
	}

	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("error preparando archivos estáticos: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))
	mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir(imagesDir))))
	mux.HandleFunc("POST /books", handleCreateBook)
	mux.HandleFunc("POST /books/{id}/edit", handleEditBook)
	mux.HandleFunc("POST /books/{id}/delete", handleDeleteBook)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		booksMu.Lock()
		books, err := loadBooks(booksFile)
		booksMu.Unlock()
		if err != nil {
			log.Printf("error cargando %s: %v", booksFile, err)
			http.Error(w, "Error interno: no se pudo cargar el catalogo de libros", http.StatusInternalServerError)
			return
		}
		data := PageData{Hostname: hostname, Books: books, Count: len(books)}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
			log.Printf("error renderizando plantilla: %v", err)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Bookzilla (host=%s) escuchando en el puerto %s", hostname, port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

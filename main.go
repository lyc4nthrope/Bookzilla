// Bookzilla: catálogo web de libros para la actividad "Desarrollo de una
// aplicación web en Golang" de Computación en la Nube (Universidad del Quindío).
package main

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
)

// Plantillas, estáticos y semilla van dentro del binario (go:embed) para que
// desplegar sea copiar un solo archivo a la VM o al contenedor. Los datos y
// las portadas NO se embeben: viven en disco para poder editarlos sin recompilar.

//go:embed templates
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

//go:embed seed
var seedFS embed.FS

// Group son los datos del grupo de trabajo que el enunciado exige mostrar
// en la página. Se declaran una sola vez y la plantilla los usa en el
// encabezado y en el banner inferior.
type Group struct {
	Members    []string
	Course     string
	Professor  string
	University string
	Program    string
	Term       string
}

var group = Group{
	Members: []string{
		"Cristhian Eduardo Osorio Restrepo",
		"Daniel Stiven Perez Cordoba",
		"William Carmona Diaz",
	},
	Course:     "Computación en la Nube",
	Professor:  "Ing. Carlos Eduardo Gómez Montoya",
	University: "Universidad del Quindío",
	Program:    "Ingeniería de Sistemas y Computación",
	Term:       "2026-2",
}

// PageData es todo lo que la plantilla index.html necesita para renderizar.
type PageData struct {
	Hostname string
	Group    Group
	Books    []Book
	Count    int
}

func main() {
	// El enunciado pide que el hostname salga del sistema operativo del host
	// que atiende la solicitud. Se lee una vez: no cambia mientras el proceso
	// vive, y detrás de un balanceador identifica qué VM respondió.
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("no se pudo obtener el hostname del sistema: %v", err)
		hostname = "desconocido"
	}

	if err := ensureSeed(); err != nil {
		log.Fatalf("error inicializando archivos de datos: %v", err)
	}

	// Validación temprana: si el JSON está mal escrito, es mejor fallar al
	// arrancar (con el motivo en el log) que servir una página rota.
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
	// CSS, JS y Bootstrap salen del binario; las portadas salen de la carpeta
	// images/ en disco (recursos independientes del HTML, como pide el enunciado).
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
		// Se relee el JSON en cada solicitud: así un libro agregado con un
		// editor de texto aparece al recargar, sin reiniciar el servidor.
		booksMu.Lock()
		books, err := loadBooks(booksFile)
		booksMu.Unlock()
		if err != nil {
			log.Printf("error cargando %s: %v", booksFile, err)
			http.Error(w, "Error interno: no se pudo cargar el catalogo de libros", http.StatusInternalServerError)
			return
		}
		data := PageData{Hostname: hostname, Group: group, Books: books, Count: len(books)}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
			log.Printf("error renderizando plantilla: %v", err)
		}
	})

	// El puerto se configura por variable de entorno para que el mismo
	// binario sirva en una VM, un contenedor o detrás de un balanceador.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Bookzilla (host=%s) escuchando en el puerto %s", hostname, port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

# Etapa 1: compila un binario estático (sin CGO) que incluye plantillas,
# estáticos y la semilla gracias a go:embed.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/bookzilla .

# Etapa 2: imagen mínima solo con el binario. data/ e images/ los crea la app
# al arrancar; montar volúmenes ahí para que sobrevivan al contenedor.
FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/bookzilla /app/bookzilla
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/bookzilla"]

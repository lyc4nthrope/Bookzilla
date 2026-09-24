FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o /out/bookzilla .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/bookzilla /app/bookzilla
COPY data /app/data
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/bookzilla"]

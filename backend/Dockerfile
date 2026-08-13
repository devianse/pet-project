# --- build stage ---
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -o /out/createuser ./cmd/createuser
RUN CGO_ENABLED=0 go build -o /out/grantaccess ./cmd/grantaccess

# --- runtime stage ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/createuser /usr/local/bin/createuser
COPY --from=build /out/grantaccess /usr/local/bin/grantaccess
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]

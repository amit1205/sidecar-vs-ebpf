
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod tidy || true
COPY . .
RUN go mod download
COPY cmd ./cmd
RUN CGO_ENABLED=0 go build -o /out/app ./cmd/app

FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
WORKDIR /home/app
COPY --from=build /out/app /usr/local/bin/app
USER root
RUN mkdir -p /var/log/app && chown -R app:app /var/log/app
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/app"]

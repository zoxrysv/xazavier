FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/xazavier ./cmd/Server

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/xazavier /app/xazavier
EXPOSE 8080
ENTRYPOINT ["/app/xazavier"]

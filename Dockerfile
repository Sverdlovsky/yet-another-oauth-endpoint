FROM golang:alpine AS build

WORKDIR /app

COPY go.sum .
COPY go.mod .

RUN go mod download

COPY cmd cmd

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o yaoae ./cmd


FROM scratch

COPY --from=build /app/yaoae /yaoae

EXPOSE 8080

ENTRYPOINT ["/yaoae"]


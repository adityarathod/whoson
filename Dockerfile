FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /whoson-server ./cmd/whoson-server

FROM alpine:3
COPY --from=build /whoson-server /whoson-server
COPY ouiDB.json /ouiDB.json
ENV OUI_DB=/ouiDB.json
EXPOSE 8080
ENTRYPOINT ["/whoson-server"]

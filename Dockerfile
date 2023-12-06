
FROM golang:1.21-alpine as builder
WORKDIR /src
COPY go.mod go.sum /src/
RUN go mod download
COPY cmd/updater /src/cmd/updater
RUN go build cmd/updater/main.go

# Copy the binary from the build container to the final container
FROM scratch
COPY --from=builder /src/main /updater
CMD ["./updater"]

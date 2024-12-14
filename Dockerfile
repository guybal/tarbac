FROM golang:1.20 as builder
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o manager main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /workspace/manager /
USER 65532:65532
ENTRYPOINT ["/manager"]

# Build the optimized emulator from source
FROM golang:1.25-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
ENV GOTOOLCHAIN=auto
RUN go mod download
COPY . .
RUN CGO_CFLAGS="-std=gnu99" CGO_ENABLED=1 go build -o /emulator ./cmd/emulator

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /emulator /bin/app
ENTRYPOINT ["/bin/app"]

EXPOSE 8888 3569 8080

CMD [ \
  "emulator", "start", \
  "--fork-host", "access.mainnet.nodes.onflow.org:9000", \
  "--skip-tx-validation", \
  "--chain-id", "mainnet", \
  "--rest-port", "8888", \
  "--port", "3569", \
  "--block-time", "0", \
  "-v" \
]

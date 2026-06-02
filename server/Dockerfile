FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod ./
# Only copy go.sum if it exists
COPY go.sum* ./

RUN go mod download

COPY . .

RUN go build -o benchmarker cmd/api/main.go

# Final stage needs docker CLI for Worker node to execute docker run
FROM docker:cli

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /app/benchmarker .
COPY Dockerfile.sandbox .
COPY cmd/director/dummy_submissions ./cmd/director/dummy_submissions

EXPOSE 9000

CMD ["./benchmarker"]

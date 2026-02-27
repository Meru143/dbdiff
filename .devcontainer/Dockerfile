FROM golang:1.21-bookworm

RUN apt-get update && apt-get install -y \
    git \
    curl \
    gcc \
    make \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY . .
RUN go mod download

ENV PATH="/root/go/bin:${PATH}"

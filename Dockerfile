# Support setting various labels on the final image
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

# Build Gavn in a stock Go builder container
FROM golang:1.16-alpine as builder

RUN apk add --no-cache gcc musl-dev linux-headers git

ADD . /go-avalanria
RUN cd /go-avalanria && go run build/ci.go install ./cmd/gavn

# Pull Gavn into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /go-avalanria/build/bin/gavn /usr/local/bin/

EXPOSE 8545 8546 30303 30303/udp
ENTRYPOINT ["gavn"]

# Add some metadata labels to help programatic image consumption
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

LABEL commit="$COMMIT" version="$VERSION" buildnum="$BUILDNUM"

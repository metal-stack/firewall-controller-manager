# Build the firewall-controller-manager binary
FROM golang:1.27 AS builder

WORKDIR /work
COPY . .
RUN make

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=builder /work/bin/firewall-controller-manager /firewall-controller-manager
ENTRYPOINT ["/firewall-controller-manager"]

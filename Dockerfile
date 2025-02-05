# Build the firewall-controller-manager binary
FROM golang:1.23 as builder

WORKDIR /work
COPY . .
RUN make

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /work/bin/firewall-controller-manager .
ENTRYPOINT ["/firewall-controller-manager"]

# Build the firewall-controller-manager binary
FROM golang:1.21 as builder

WORKDIR /work
COPY . .
RUN make

FROM alpine:3.18
COPY --from=builder /work/bin/firewall-controller-manager .
USER 65534
ENTRYPOINT ["/firewall-controller-manager"]

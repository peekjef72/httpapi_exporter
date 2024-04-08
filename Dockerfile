FROM golang:1.22 AS builder

# Get httpapi_exporter
ADD .   /go/src/httpapi_exporter
WORKDIR /go/src/httpapi_exporter

# Do makefile
RUN make

# Make image and copy build httpapi_exporter
FROM        quay.io/prometheus/busybox:glibc
COPY        --from=builder /go/src/httpapi_exporter/httpapi_exporter  /bin/httpapi_exporter

EXPOSE      9321
ENTRYPOINT  [ "/bin/httpapi_exporter" ]

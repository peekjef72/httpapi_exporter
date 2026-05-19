FROM golang:1.26.3 AS builder

# 1. Installer promu (l'outil de build officiel de Prometheus)
RUN go install github.com/prometheus/promu@latest

# Récupérer les sources de l'application
ADD .   /go/src/httpapi_exporter
WORKDIR /go/src/httpapi_exporter

# 2. Compiler le binaire avec promu
# prom_build va automatiquement injecter le sha du commit, la date et la version
RUN /go/bin/promu build --prefix=/go/src/httpapi_exporter

# Image finale minimale
FROM        quay.io/prometheus/busybox:glibc
# Note le changement de chemin : promu génère par défaut le binaire à la racine ou selon ton fichier .promu.yml
COPY        --from=builder /go/src/httpapi_exporter/httpapi_exporter  /bin/httpapi_exporter

EXPOSE      9321
ENTRYPOINT  [ "/bin/httpapi_exporter" ]
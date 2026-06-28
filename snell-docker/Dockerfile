FROM debian:stable-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends curl unzip ca-certificates && \
    rm -rf /var/lib/apt/lists/*

ARG SNELL_VERSION=v5.0.1
ARG TARGETARCH=amd64

RUN curl -fsSL -o /tmp/snell.zip \
    "https://dl.nssurge.com/snell/snell-server-${SNELL_VERSION}-linux-${TARGETARCH}.zip" && \
    unzip -o /tmp/snell.zip -d /usr/local/bin/ && \
    chmod +x /usr/local/bin/snell-server && \
    rm /tmp/snell.zip && \
    apt-get remove -y curl unzip && \
    apt-get autoremove -y

EXPOSE 10000-60000

ENTRYPOINT ["/usr/local/bin/snell-server"]
CMD ["-c", "/etc/snell/snell-server.conf"]

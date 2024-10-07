FROM sigoden/dufs

ARG ARCHIVE_NAME

# Dufs settings:
ENV DUFS_BIND=127.0.0.1
ENV DUFS_PORT=5000
ENV DUFS_TLS_CERT=cert.pem
ENV DUFS_TLS_KEY=key.pem

COPY ${ARCHIVE_NAME} /data/

CMD ["/data"]

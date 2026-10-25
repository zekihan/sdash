# Build binaries and CSS with the verified build scripts before packaging.
FROM --platform=$BUILDPLATFORM busybox:1.38.0@sha256:fd7dc98638c8e305f4dc34e979f1c0fdfdcaeb0fbf8fcff77ae834b6da3d7e6e AS builder

ARG TARGETPLATFORM

COPY ./dist ./dist

RUN case ${TARGETPLATFORM} in \
        "linux/amd64")    ARCH="amd64"    ;; \
        "linux/arm64")    ARCH="arm64"    ;; \
        "linux/riscv64")  ARCH="riscv64"  ;; \
        "linux/ppc64le")  ARCH="ppc64le"  ;; \
        "linux/s390x")    ARCH="s390x"    ;; \
        "linux/386")      ARCH="386"      ;; \
        "linux/arm/v7")   ARCH="armv7"   ;; \
        "linux/arm/v6")   ARCH="armv6"   ;; \
    esac && \
    mv ./dist/sdash_linux_${ARCH}/sdash /usr/bin/sdash

FROM gcr.io/distroless/static-debian12:latest@sha256:d75cdd72874d4790092fcb1b058493ecf6bb5bf2b2b897045b00ff01d91843f2 AS final

COPY ./assets ./assets
COPY --from=builder /usr/bin/sdash /usr/bin/sdash

ENTRYPOINT ["sdash"]

LABEL org.opencontainers.image.authors="" \
    org.opencontainers.image.vendor="" \
    org.opencontainers.image.documentation="https://github.com/zekihan/sdash#readme" \
    org.opencontainers.image.url="https://github.com/zekihan/sdash" \
    org.opencontainers.image.source="https://github.com/zekihan/sdash" \
    org.opencontainers.image.licenses="AGPL-3.0" \
    org.opencontainers.image.title="SDash" \
    org.opencontainers.image.description="A simple configurable service dashboard."

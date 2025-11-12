FROM --platform=$BUILDPLATFORM docker.io/node:24-alpine@sha256:10d711afd59d7025ccc97fc4f1f78be8588a5f025d3a8b179af64f6720f5d99b AS tailwind
WORKDIR /app

RUN npm install -D tailwindcss @tailwindcss/cli @tailwindcss/postcss cssnano postcss postcss-cli

COPY build ./build
COPY views ./views
COPY assets/css ./assets/css

ENV NODE_ENV=production

RUN npx @tailwindcss/cli \
                -i ./build/tailwind/input.css \
                -o assets/css/tailwind.css

RUN npx postcss --config ./build/csspostprocess/postcss.config.js \
    assets/css/*.css \
    --base assets/css \
    --dir assets/css \
    --ext ".min.css"

FROM --platform=$BUILDPLATFORM busybox:1.37.0@sha256:e3652a00a2fabd16ce889f0aa32c38eec347b997e73bd09e69c962ec7f8732ee AS builder

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

FROM --platform=$BUILDPLATFORM zekihan/distroless:v0.1.0@sha256:3b509aaefa4a40d924099baf6ad58238a718760ec9ee8ccc1db9a0d5f406f80a AS final

COPY ./assets ./assets
COPY --from=tailwind /app/assets/css ./assets/css
COPY --from=builder /usr/bin/sdash /usr/bin/sdash

ENTRYPOINT ["sdash"]

LABEL org.opencontainers.image.authors="38537758+zekihan@users.noreply.github.com" \
    org.opencontainers.image.url="https://example.com/" \
    org.opencontainers.image.documentation="https://docs.example.com/" \
    org.opencontainers.image.source="https://github.com/zekihan/sdash" \
    org.opencontainers.image.vendor="zekihan" \
    org.opencontainers.image.licenses="AGPL-3.0" \
    org.opencontainers.image.title="Sdash" \
    org.opencontainers.image.description="Sdash is a tool to help you optimize your dashboards for speed and cost."

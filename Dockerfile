FROM --platform=$BUILDPLATFORM docker.io/node:24-alpine@sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1 AS tailwind
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

FROM --platform=$BUILDPLATFORM busybox:1.38.0@sha256:dc2d74b28e4cf8984fa52af1f39bc7c3d9c73760b41a74d629f5d11b1ab28616 AS builder

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

FROM --platform=$BUILDPLATFORM zekihan/distroless:v1.0.0@sha256:5c5ca36f21605d0fee2a36141f3ce67f941c2b02bd4356db5654704a804f66cb AS final

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

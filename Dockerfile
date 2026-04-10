FROM --platform=$BUILDPLATFORM docker.io/node:24-alpine@sha256:01743339035a5c3c11a373cd7c83aeab6ed1457b55da6a69e014a95ac4e4700b AS tailwind
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

FROM --platform=$BUILDPLATFORM busybox:1.37.0@sha256:1487d0af5f52b4ba31c7e465126ee2123fe3f2305d638e7827681e7cf6c83d5e AS builder

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

FROM --platform=$BUILDPLATFORM zekihan/distroless:v1.0.0@sha256:6524078396003523804b9f1c1a008e919e8a9a193193d338d192e4231f3276fa AS final

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

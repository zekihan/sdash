FROM --platform=$BUILDPLATFORM docker.io/node:24-alpine@sha256:cd6fb7efa6490f039f3471a189214d5f548c11df1ff9e5b181aa49e22c14383e AS tailwind
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

FROM --platform=$BUILDPLATFORM busybox:1.37.0@sha256:e226d6308690dbe282443c8c7e57365c96b5228f0fe7f40731b5d84d37a06839 AS builder

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

FROM --platform=$BUILDPLATFORM zekihan/distroless:v1.0.0@sha256:813fffdeba748646d6becce13b12969c8107cc9cf88fe130798952600506ed10 AS final

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

# First stage: Tailwind CSS build
FROM docker.io/node:23-alpine@sha256:1467ea23cce893347696b155b9e00e7f0ac7d21555eb6f27236f1328212e045e AS tailwind
WORKDIR /app

RUN npm install -D tailwindcss cssnano postcss postcss-cli

COPY build ./build
COPY views ./views
COPY assets/css ./assets/css

ENV NODE_ENV=production

RUN npx tailwindcss --config ./build/tailwind/tailwind.config.js \
                -i ./build/tailwind/input.css \
                -o assets/css/tailwind.css

RUN npx postcss --config ./build/csspostprocess/postcss.config.js \
    assets/css/*.css \
    --base assets/css \
    --dir assets/css \
    --ext ".min.css"

FROM ghcr.io/a-h/templ:v0.2.793@sha256:abfa7f0592109f01fe492f922ad80d4272c6aefc282d0d9f82ffc67e244d403f AS templ
COPY --chown=65532:65532 . /app
WORKDIR /app
RUN ["templ", "generate", "-path", "./views"]

FROM docker.io/golang:1.23.3@sha256:d56c3e08fe5b27729ee3834854ae8f7015af48fd651cd25d1e3bcf3c19830174 AS builder
COPY --from=templ /app /app
WORKDIR /app

COPY go.mod .
COPY go.sum .

ENV CGO_ENABLED=0 GO111MODULE=on GOOS=linux

RUN go mod download

COPY . .

RUN ./scripts/build.sh

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY ./assets ./assets
COPY --from=tailwind /app/assets/css ./assets/css
COPY --from=builder /app/tmp/bin/sdash ./

ENTRYPOINT ["./sdash"]

# First stage: Tailwind CSS build
FROM docker.io/node:23-alpine@sha256:d03e75e7ba1385c2944f4cc374eb5abe0715234f87da5121dbd64f7262ad10df AS tailwind
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

FROM docker.io/golang:1.23.3@sha256:b4aabba13d38c069fa6d6ee3656e1fcb11c18181ecfd13728bfced414f6422ca AS builder
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

# First stage: Tailwind CSS build
FROM docker.io/node:23-alpine@sha256:c61b6b12a3c96373673cd52d7ecee2314e82bca5d541eecf0bc6aee870c8c6f7 AS tailwind
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

FROM docker.io/golang:1.23.4@sha256:fffeec553ab94798ca27bda80814acd3f776492d7deafb1a82dd08830b76ec7c AS builder
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

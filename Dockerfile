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

FROM ghcr.io/a-h/templ:v0.3.819@sha256:e1655b722c17a0537a4525f842c1c3cb6fd468679eeffa05b370d9e85777db75 AS templ
COPY --chown=65532:65532 . /app
WORKDIR /app
RUN ["templ", "generate", "-path", "./views"]

FROM docker.io/golang:1.23.4@sha256:7ea4c9dcb2b97ff8ee80a67db3d44f98c8ffa0d191399197007d8459c1453041 AS builder
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

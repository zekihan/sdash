# First stage: Tailwind CSS build
FROM docker.io/node:22-alpine@sha256:c9bb43423a6229aeddf3d16ae6aaa0ff71a0b2951ce18ec8fedb6f5d766cf286 AS tailwind
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

FROM ghcr.io/a-h/templ:v0.2.778@sha256:5e9a7c306c62b6d7b5d5817f295545b69c24024c24cc89c2a9ae3dd5dcdcb463 AS templ
COPY --chown=65532:65532 . /app
WORKDIR /app
RUN ["templ", "generate", "-path", "./views"]

FROM docker.io/golang:1.23.2@sha256:a7f2fc9834049c1f5df787690026a53738e55fc097cd8a4a93faa3e06c67ee32 AS builder
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

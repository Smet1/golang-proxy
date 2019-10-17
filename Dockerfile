FROM golang:alpine AS build

# Копируем исходный код в Docker-контейнер
WORKDIR /server
COPY . .

RUN go build cmd/proxy/main.go

# Копируем на чистый образ
FROM alpine

COPY --from=build /server/main /main
COPY --from=build /server/config.yaml /config.yaml
COPY --from=build /server/ca-cert.pem /ca-cert.pem
COPY --from=build /server/ca-key.pem /ca-key.pem


RUN ls

# Объявлем порт сервера
EXPOSE 8888

CMD ./main
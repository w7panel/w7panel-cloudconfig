FROM golang:1.25 AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/w7panel-cloudconfig .

FROM node:22 AS frontend
WORKDIR /src/ui
COPY ui/package*.json ./
RUN npm ci
COPY ui/ .
RUN npm run build

FROM alpine:3.22
WORKDIR /
COPY --from=backend /out/w7panel-cloudconfig /w7panel-cloudconfig
COPY --from=frontend /src/ui/dist /kodata
EXPOSE 8001 8081 18080
ENTRYPOINT ["/w7panel-cloudconfig"]

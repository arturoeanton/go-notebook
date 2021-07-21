FROM golang:1.16.5-alpine 
WORKDIR /app
ENV CGO_ENABLED=1
RUN apk add build-base
COPY . .
RUN go mod tidy
RUN go build .
RUN go get -u gonum.org/v1/plot
RUN go get -u gonum.org/v1/plot/vg
RUN go get -u gonum.org/v1/plot/plotter
RUN go get -u gonum.org/v1/plot/plotutil
RUN go get -u github.com/google/uuid


CMD ["/app/go-notebook"]

## podman login docker.io
## podman build  -t arturoeanton/go-notebook  .
## podman push arturoeanton/go-notebook
## podman run --rm -p 1323:1323 go-notebook
FROM golang:1.26-alpine
RUN apk add --no-cache build-base
WORKDIR /app
COPY . .
RUN go mod tidy
RUN go build .
RUN go get gonum.org/v1/plot
RUN go get gonum.org/v1/plot/vg
RUN go get gonum.org/v1/plot/plotter
RUN go get gonum.org/v1/plot/plotutil


CMD ["/app/go-notebook"]

## docker login docker.io
## docker build .  -t arturoeanton/go-notebook
## docker push arturoeanton/go-notebook
## docker run --rm -p 1323:1323 go-notebook

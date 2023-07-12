FROM golang:1.20-alpine 
WORKDIR /app
COPY . .
RUN go mod tidy
RUN go build .
RUN go get -u gonum.org/v1/plot
RUN go get -u gonum.org/v1/plot/vg
RUN go get -u gonum.org/v1/plot/plotter
RUN go get -u gonum.org/v1/plot/plotutil
RUN go get -u github.com/google/uuid


CMD ["/app/go-notebook"]

## docker login docker.io
## docker build .  -t arturoeanton/go-notebook  
## docker push arturoeanton/go-notebook
## docker run --rm -p 1323:1323 go-notebook
FROM golang:1.20-alpine AS builder

WORKDIR /work

ENV CGO_CXXFLAGS="-std=c++11"
ENV GOPRIVATE=github.com/abilitylab
ENV GO111MODULE="on"

RUN apk update
RUN apk add git g++ make wget

COPY .gitconfig* /work/

RUN if [[ -f .gitconfig ]] ; then cp .gitconfig /root/.gitconfig ; else echo '.gitconfig is missing, build can fail'; fi

COPY pkg/graph/pkg/hnsw /work/pkg/graph/pkg/hnsw/

RUN cd /work/pkg/graph/pkg/hnsw && make
RUN cp /work/pkg/graph/pkg/hnsw/libhnsw.so /usr/local/lib
RUN ldconfig /usr/local/lib

COPY . /work/

RUN #mkdir /work/output
RUN go build -o output/graph ./cmd/graph/main.go
RUN chmod +x output/graph



FROM alpine:latest

RUN apk update && apk --no-cache add openssl ca-certificates libc6-compat libstdc++ wget && rm -rf /var/cache/apk/*

WORKDIR /app

COPY --from=builder /work/pkg/graph/pkg/hnsw/libhnsw.so /usr/local/lib/
RUN ldconfig /usr/local/lib
COPY --from=builder /work/output/graph /app/

EXPOSE 8080/tcp
EXPOSE 8080/udp

ENTRYPOINT ["/app/graph"]

CMD []

# Build container;
FROM openfalcon/makegcc-golang:1.15-alpine
LABEL maintainer laiwei.ustc@gmail.com
USER root

ENV http_proxy=http://11.122.78.49:3128
ENV https_proxy=http://11.122.78.49:3128
ENV HTTP_PROXY=$http_proxy
ENV HTTPS_PROXY=$https_proxy
ENV no_proxy=.aliyun.com
ENV FALCON_DIR=/open-falcon PROJ_PATH=${GOPATH}/src/github.com/open-falcon/falcon-plus

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    mkdir -p $FALCON_DIR && \
    mkdir -p $FALCON_DIR/logs && \
    apk add --no-cache ca-certificates bash git supervisor
COPY . ${PROJ_PATH}

WORKDIR ${PROJ_PATH}
RUN make all \
    && make pack4docker \
    && tar -zxf open-falcon-v*.tar.gz -C ${FALCON_DIR} \
    && rm -rf ${PROJ_PATH}

# Final container;
FROM alpine:3.13
LABEL maintainer laiwei.ustc@gmail.com
USER root

ENV FALCON_DIR=/open-falcon

RUN mkdir -p $FALCON_DIR/logs && \
    apk add --no-cache ca-certificates bash git supervisor

ADD docker/supervisord.conf /etc/supervisord.conf

COPY --from=0 ${FALCON_DIR} ${FALCON_DIR}

EXPOSE 8433 8080 18433 6030
WORKDIR ${FALCON_DIR}

CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]

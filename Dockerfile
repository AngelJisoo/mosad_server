# 首先利用golang镜像来编译出可执行文件，之后再转换到较小镜像
FROM golang:alpine as base
RUN mkdir /build
COPY . /build
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=windows GOARCH=amd64 GO111MODULE=on go build -o server .
# 这条RUN先是设置了env然后调用go build编译所有文件，生成可执行文件server


#FROM scratch：官方说明：该镜像是一个空的镜像，可以用于构建超小镜像，把一个可执行文件扔进来直接执行
FROM scratch
# 拷贝/build/server文件到当前目录下，并RUN执行
COPY --from=base /build/server /
CMD ["/server"]
FROM golang:1.25-alpine as build

# 容器环境变量添加，会覆盖默认的变量值
ENV GO111MODULE=on

# 新增构建依赖包，更新源，安装时区工具
RUN apk update && apk add --no-cache tzdata build-base upx git

# 设置时区为上海
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && echo "Asia/Shanghai" > /etc/timezone
ENV TZ=Asia/Shanghai

# 设置工作区
WORKDIR /go/release

# 再把全部文件添加到/go/release目录 这样就可以缓存go mod download
COPY . .

RUN go mod download
RUN go mod tidy

# 编译：把cmd/main.go编译成可执行的二进制文件，命名为app
RUN go build -tags pro -ldflags "-s -w" -o app

# 使用upx再次压缩 -1到9 速度依次变慢
RUN upx -9 app

FROM alpine:3



COPY --from=build /etc/ssl/certs /etc/ssl/certs
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && echo "Asia/Shanghai" >  /etc/timezone



RUN mkdir /code
WORKDIR /code

COPY --from=build /go/release/app /code
COPY --from=build /go/release/assets /code/assets
COPY --from=build /go/release/templates /code/templates
COPY --from=build /go/release/locales /code/locales


RUN chmod -R 777 /code
# 运行容器执行时的口令1
CMD ["./app"]
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
RUN adduser -D -s /bin/sh qnap-vm

WORKDIR /home/qnap-vm

COPY qnap-vm .

USER qnap-vm

ENTRYPOINT ["./qnap-vm"]
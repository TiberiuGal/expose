#FROM ubuntu:20.04
FROM scratch

COPY bin/cloudy cloudy

EXPOSE 8080 1044

ENTRYPOINT [ "./cloudy" ]

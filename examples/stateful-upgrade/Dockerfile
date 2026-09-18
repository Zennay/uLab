FROM alpine:3.20
ARG APP_VERSION=v1
ENV APP_VERSION=$APP_VERSION
COPY fixture.sh /usr/local/bin/fixture
RUN chmod +x /usr/local/bin/fixture
ENTRYPOINT ["fixture"]

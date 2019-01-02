FROM stackbrew/ubuntu:16.04

RUN mkdir -p /srv
COPY build/cedar /srv

EXPOSE 3000
CMD ["./srv/cedar", "service", "--port", "3000"]

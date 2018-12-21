FROM stackbrew/ubuntu:16.04

RUN mkdir -p /app
RUN apt-get update && apt-get install -y curl
RUN cd /app && curl https://s3.amazonaws.com/mciuploads/sink/sink_ubuntu1604_123ff0df6f9ce93cfcc6297a04f2c911c9c4be17_18_12_17_22_06_44/cedar-dist-%7Brevision%7D.tar.gz | tar -zxvf -

EXPOSE 3000
CMD ["./app/cedar", "service", "--port", "3000"]

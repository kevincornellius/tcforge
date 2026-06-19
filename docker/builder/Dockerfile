FROM ubuntu:22.04

RUN apt-get update && apt-get install -y \
    g++ \
    cmake \
    make \
    git \
    && rm -rf /var/lib/apt/lists/*

# Build tcframe from source
RUN git clone https://github.com/ia-toki/tcframe.git /tcframe && \
    cd /tcframe && \
    cmake . && \
    make install

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /contest
ENTRYPOINT ["/entrypoint.sh"]

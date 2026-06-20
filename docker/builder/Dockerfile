FROM ubuntu:22.04

RUN apt-get update && apt-get install -y \
    g++ \
    git \
    python3 \
    && rm -rf /var/lib/apt/lists/*

# tcframe is header-only — just clone and point TCFRAME_HOME at it
RUN git clone --depth=1 https://github.com/ia-toki/tcframe.git /tcframe
ENV TCFRAME_HOME=/tcframe
RUN ln -s /tcframe/scripts/tcframe /usr/local/bin/tcframe

COPY entrypoint.sh /entrypoint.sh
COPY parse_spec.py /parse_spec.py
RUN chmod +x /entrypoint.sh

WORKDIR /contest
ENTRYPOINT ["/entrypoint.sh"]

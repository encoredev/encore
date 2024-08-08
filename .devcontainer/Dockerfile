FROM golang:1.21.0

RUN apt-get update && apt-get install -y sudo
RUN curl -fsSL https://deb.nodesource.com/setup_16.x | sudo -E bash - && \
	apt-get install -y nodejs npm

ADD scripts /scripts
RUN bash /scripts/install.sh
RUN bash /scripts/godeps.sh

ENV ENCORE_GOROOT=/encore-release/encore-go

# syntax=docker/dockerfile:1.4
FROM --platform=$TARGETPLATFORM ubuntu:22.04 AS build
ARG TARGETPLATFORM
ARG RELEASE_VERSION
RUN mkdir /encore
ADD rename-binary-if-needed.bash rename-binary-if-needed.bash
ADD artifacts /artifacts
RUN /bin/bash -c 'SRC=encore-$(echo $TARGETPLATFORM | tr '/' '_'); tar -C /encore -xzf /artifacts/$SRC.tar.gz'
RUN /bin/bash rename-binary-if-needed.bash

FROM --platform=$TARGETPLATFORM ubuntu:22.04
RUN apt-get update && apt-get install -y -f ca-certificates
ENV PATH="/encore/bin:${PATH}"
WORKDIR /src
ADD encore-entrypoint.bash /bin/encore-entrypoint.bash
ENTRYPOINT ["/bin/encore-entrypoint.bash"]
COPY --from=build /encore /encore

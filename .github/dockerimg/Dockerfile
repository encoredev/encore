# syntax=docker/dockerfile:1.4
FROM --platform=$BUILDPLATFORM ubuntu:22.04 AS build
ARG TARGETPLATFORM
ARG RELEASE_VERSION
ADD artifacts /artifacts
RUN mkdir /encore
RUN /bin/bash -c 'SRC=encore-$RELEASE_VERSION-$(echo $TARGETPLATFORM | tr '/' '_'); tar -C /encore -xzf /artifacts/$SRC/$SRC.tar.gz'

FROM --platform=$BUILDPLATFORM ubuntu:22.04
RUN apt-get update && apt-get install -y -f ca-certificates
ENV PATH="/encore/bin:${PATH}"
WORKDIR /src
ADD encore-entrypoint.bash /bin/encore-entrypoint.bash
COPY --from=build /encore /encore
ENTRYPOINT ["/bin/encore-entrypoint.bash"]

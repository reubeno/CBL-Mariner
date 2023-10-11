FROM mcr.microsoft.com/onebranch/cbl-mariner/build:2.0

# Create dirs we'll later bind-mount over.
RUN mkdir -m 777 -p /sources && \
    ln -s /usr/src/linux /sources && \
    for i in $(seq 1 12); do \
        mkdir -m 777 -p /temp/DockerStage/docker-chroot-$i/dev/pts; \
        mkdir -m 777 -p /temp/DockerStage/docker-chroot-$i/proc; \
        mkdir -m 777 -p /temp/DockerStage/docker-chroot-$i/sys; \
        mkdir -m 777 -p /temp/DockerStage/docker-chroot-$i/run; \
        mkdir -m 777 -p /temp/DockerStage/docker-chroot-$i/localrpms/noarch; \
        mkdir -m 777 -p /temp/DockerStage/docker-chroot-$i/localrpms/$(uname -p); \
        mkdir -m 777 -p /temp/DockerStage/docker-chroot-$i/localrpms/upstream-cached-rpms; \
    done

# Set up locale settings.
ENV TZ=America/Los_Angeles
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# Install packages required as prerequisites by Mariner tooling.
RUN tdnf install -qy dnf dnf-plugins-core && \
    dnf -y install \
        acl \
        binutils \
        bison \
        cdrkit \
        curl \
        dnf-utils \
        dosfstools \
        gawk \
        gcc \
        git \
        glibc-devel \
        golang \
        kernel-headers \
        make \
        moby-cli \
        parted \
        pigz \
        python3 \
        qemu-img \
        rpm \
        rpm-build \
        rsync \
        sudo \
        tar \
        wget

# Add a non-root user that we'll do our best to use for development.
RUN useradd -m devel && \
    echo "devel ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# We switch to using the non-root user for remaining operations.
USER devel

ENV GOPATH="/home/devel/go"
ENV PATH="$PATH:/home/devel/go/bin"

# Install go packages required by and/or useful to the VSCode go extension.
RUN go install -v golang.org/x/tools/gopls@latest && \
    go install -v github.com/rogpeppe/godef@latest && \
    go install -v github.com/ramya-rao-a/go-outline@latest && \
    go install -v github.com/go-delve/delve/cmd/dlv@latest && \
    go install -v honnef.co/go/tools/cmd/staticcheck@latest && \
    go install -v github.com/josharian/impl@latest && \
    go install -v github.com/fatih/gomodifytags@latest && \
    go install -v github.com/haya14busa/goplay/cmd/goplay@latest && \
    go install -v github.com/cweill/gotests/gotests@latest

# Update shell defaults.
COPY welcome.txt /home/devel/welcome.txt
COPY initialize-container-shell.sh /home/devel/initialize-container-shell.sh
RUN echo "source $HOME/initialize-container-shell.sh" >> /home/devel/.bashrc

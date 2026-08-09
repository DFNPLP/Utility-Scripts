FROM golang:1.24.1-bookworm

# todo remove this and rely on mounts and env vars
# COPY ["./go.sum", "./go.mod", "./pkg/*", "./cmd/fimulcrumuifyne/*", "./"] 
# COPY ["./pkg/*", "./pkg/"] 
# COPY ["./cmd/fimulcrumuifyne/*", "./cmd/fimulcrumuifyne/"] 


# https://docs.fyne.io/started/cross-compiling.html, need to adopt this, the below probably isn't right
# go install fyne.io/fyne/v2/cmd/fyne@latest
# go install github.com/fyne-io/fyne-cross@latest
# cd /out
# doesn't seem to work????
#fyne-cross windows -arch=* -output /out/ui /repo/cmd/fimulcrumuifyne


# cross compiling notes https://stackoverflow.com/questions/74334529/go-fyne-project-cant-cross-compile-from-linux-to-windows
RUN mkdir /repo
RUN mkdir /out
RUN mkdir /temp
RUN echo 'go build /repo/$BUILD_PATH -o /out/fimulcrum_fyne' >> /temp/build.sh
RUN chmod +x /temp/build.sh

ENTRYPOINT ["sh", "/temp/build.sh"]

# Specify GOOS=target-OS GOARCH=target-architecture to build specific archs, https://go.dev/doc/install/source#environment
# todo???? use read only volume mount? and also a mount for where to build the file?
# add instructions, you should build from root of repo (not actual root, but fimulcrum root)


## turns out all this is pointless? fyne-cross just uses docker anyway to run the compilation?
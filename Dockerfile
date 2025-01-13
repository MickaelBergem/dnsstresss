# Use the official Golang image as a build environment
FROM golang:1.20 as builder

# Set the working directory in the container
WORKDIR /app

# Copy the source code into the container
COPY . .

# Download the dependencies and build the binary
RUN go mod tidy && go build -o /dnsstress .

# Use a newer base image with GLIBC 2.32 or above (e.g., ubuntu:20.04)
FROM ubuntu:latest

# Install necessary libraries for running the binary (including libc)
RUN apt-get update && apt-get install -y \
    ca-certificates \
    libc6 \
    && rm -rf /var/lib/apt/lists/*

# Set the working directory for the runtime image
WORKDIR /root/

# Copy the compiled binary from the builder image
COPY --from=builder /dnsstress /dnsstress

# Command to run the dnsstress binary
ENTRYPOINT ["/dnsstress"]

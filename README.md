# Goci Wrapper

Goci Wrapper is a dynamic OCI image wrapper service that allows you to add functionality to arbitrary container images without rebuilding them. It's particularly useful for CI/CD systems like Woodpecker CI where you need to mark images as "safe" while adding custom functionality.

## Use Case

The primary use case for Goci Wrapper is with Woodpecker CI, which allows defining "safe" images. Sometimes you need to add features to arbitrary known images without having to rebuild them, while also marking them as safe for Woodpecker CI.

With Goci Wrapper, you can:
```bash
docker pull localhost:5000/wrap/mirror.gcr.io/woodpeckerci/plugin-ready-release-go/3.4.0/with/registry.yewolf.fr/test/test/latest
```

This creates a wrapped image that Woodpecker CI recognizes as safe, combining the functionality of both the base image and your wrapper.

## How It Works

1. **Dynamic Wrapping**: The service intercepts Docker registry requests with a special path format
2. **Layer Composition**: It pulls both the base image and wrapper image, then combines their layers
3. **Entrypoint Modification**: The wrapper's entrypoint becomes the new entrypoint, with the original entrypoint becoming arguments
4. **Caching**: Wrapped images are cached to avoid redundant processing
5. **Registry Proxy**: Acts as a registry proxy for the wrapped images

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Docker Pull   │───▶│  Goci Wrapper    │───▶│  Memory Registry│
│                 │    │     Service      │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                               │
                               ▼
                       ┌──────────────────┐
                       │ Remote Registries│
                       │ (pull base +     │
                       │  wrapper images) │
                       └──────────────────┘
```

## Building and Running

### Prerequisites
- Go 1.24.4 or later
- Docker

### Build the Service
```bash
git clone <your-repo>
cd goci-wrapper
go build -o goci-wrapper .
```

### Run the Service
```bash
./goci-wrapper
```

The service will start on port 5000 by default and log:
```
Starting in-memory registry
Goci Wrapper server starting on :5000
```

## Creating Wrapper Images

A wrapper image is a container image that contains additional functionality you want to add to base images. To create a wrapper image:

### 1. Create a Wrapper Script

Create a script that will become the new entrypoint. This script should eventually call the original image's entrypoint.

Example `wrapper.sh`:
```bash
#!/bin/bash

# Add your custom functionality here
echo "Running custom wrapper functionality..."

# Add environment setup, logging, monitoring, etc.
export CUSTOM_VAR="wrapped"

# Execute the original command
exec "$@"
```

### 2. Create a Dockerfile for Your Wrapper

```dockerfile
FROM scratch

# Copy your wrapper script
COPY wrapper.sh /usr/local/bin/wrapper.sh

# IMPORTANT: Add the label that tells goci-wrapper where the script is
LABEL org.goci.wrapper="/usr/local/bin/wrapper.sh"
```

### 3. Build and Push the Wrapper Image

```bash
docker build -t your-registry.com/my-wrapper:latest .
docker push your-registry.com/my-wrapper:latest
```

### Key Requirements for Wrapper Images

1. **Label**: Must include the `org.goci.wrapper` label pointing to the wrapper script
2. **Executable**: The wrapper script must be executable
3. **Script Logic**: The script should execute the original command using `exec "$@"`

## Usage Examples

### Basic Usage

Pull a wrapped image:
```bash
docker pull localhost:5000/wrap/ubuntu/22.04/with/your-registry.com/my-wrapper/latest
```

This creates an image that:
- Has all layers from `ubuntu:22.04`
- Adds the layer from `your-registry.com/my-wrapper:latest`
- Uses your wrapper script as the entrypoint
- Passes the original Ubuntu entrypoint as arguments to your wrapper

### Woodpecker CI Integration

In your Woodpecker CI configuration:

```yaml
# .woodpecker.yml
pipeline:
  build:
    image: localhost:5000/wrap/mirror.gcr.io/woodpeckerci/plugin-ready-release-go/3.4.0/with/registry.yewolf.fr/test/test/latest
    commands:
      - echo "This runs with both the original plugin functionality and your custom wrapper"
      - go build ./...
```

### Complex Wrapper Example

Create a wrapper that adds monitoring and logging:

```bash
#!/bin/bash
# monitoring-wrapper.sh

# Set up logging
exec 1> >(tee -a /var/log/wrapper.log)
exec 2>&1

echo "$(date): Starting wrapped command: $*"

# Add monitoring
start_time=$(date +%s)

# Set up signal handlers
cleanup() {
    end_time=$(date +%s)
    duration=$((end_time - start_time))
    echo "$(date): Command completed in ${duration}s"
}
trap cleanup EXIT

# Execute original command
exec "$@"
```

## API Reference

### Path Format

```
/v2/wrap/{upstream-image}/with/{wrapper-image}/manifests/{tag}
```

Where:
- `upstream-image`: The base image path (e.g., `ubuntu/22.04`)
- `wrapper-image`: Your wrapper image path (e.g., `registry.example.com/my-wrapper/latest`)
- `tag`: The tag for the resulting wrapped image

### Configuration

Environment variables:
- `SERVER_PORT`: Port to run on (default: `:5000`)
- `MEMORY_NETWORK`: Memory network name (default: `memu`)
- `MEMORY_ADDRESS`: Memory address (default: `goci-wrapper-registry`)

### Labels

Wrapper images must include:
- `org.goci.wrapper`: Path to the wrapper script inside the container

## Troubleshooting

### Common Issues

1. **"wrapper image missing 'org.goci.wrapper' label"**
   - Ensure your wrapper image has the required label
   - Check the label points to an existing file in the image

2. **"wrapper image has no layers"**
   - Your wrapper image needs at least one layer with the wrapper script
   - Avoid using completely empty images

3. **"invalid upstream image format"**
   - Upstream and wrapper image paths must contain at least one `/`
   - Use full registry paths when needed

### Debug Mode

Run with verbose logging:
```bash
./goci-wrapper 2>&1 | tee goci-wrapper.log
```

## Security Considerations

- The service runs an in-memory registry that doesn't persist images
- Wrapper scripts run with the same privileges as the original image
- Consider network policies to restrict access to the wrapper service
- Validate wrapper images before using them in production

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

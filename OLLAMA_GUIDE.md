# Instructions on how to get LLocalSearch working with your Ollama instance

<!--toc:start-->
- [Instructions on how to get LLocalSearch working with your Ollama instance](#instructions-on-how-to-get-llocalsearch-working-with-your-ollama-instance)
  - [You're running Ollama on your host machine (without docker)](#youre-running-ollama-on-your-host-machine-without-docker)
    - [You're using Linux or macOS](#youre-using-linux-or-macos)
    - [You're using Windows](#youre-using-windows)
  - [You're running Ollama in a docker container on the same machine as LLocalSearch](#youre-running-ollama-in-a-docker-container-on-the-same-machine-as-llocalsearch)
  - [You're running Ollama on a Server or different machine](#youre-running-ollama-on-a-server-or-different-machine)
<!--toc:end-->

## You're running Ollama on your host machine (without docker)

### You're using Linux or macOS

1. Make sure Ollama is listening on all interfaces (`0.0.0.0`, or at least the docker network).
2. Add the following to the `.env` file (create one if it doesn't exist) in the root of the project:

```yaml
OLLAMA_HOST=host.docker.internal:11434
```

> [!WARNING]
> Some linux users reported that this solution requires docker desktop to be installed. Please report back if that's the case for you. I don't have this issue on NixOS.

### You're using Windows

Try the above and tell me if it worked, I will update these docs.

## You're running Ollama in a docker container on the same machine as LLocalSearch 

1. Make sure your exposing Ollama on port 11434.
2. Add the following to the `.env` file (create one if it doesn't exist) in the root of the project:

```yaml
OLLAMA_HOST=host.docker.internal:11434
```

## You're running Ollama on a Server or different machine

1. Make sure Ollama is reachable from the container.
2. Add the following to the `.env` file (create one if it doesn't exist) in the root of the project:

```yaml
OLLAMA_HOST=ollama-server-ip:11434
```

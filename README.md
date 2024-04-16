# LLocalSearch

> [!IMPORTANT]
> Discuss configurations and setups with other users at: https://discord.gg/Cm77Eav5mX. Help / Support is handled exclusively on GitHub to allow people with similar issues to find solutions more easily. 

## What it is

LLocalSearch is a completely locally running search aggregator using LLM Agents. The user can ask a question and the system will use a chain of LLMs to find the answer. The user can see the progress of the agents and the final answer. No OpenAI or Google API keys are needed.

### Demo

https://github.com/nilsherzig/LLocalSearch/assets/72463901/86ab3175-ac5a-48cf-bba6-73b4380d06d8

## Features 

-  🕵️ Completely local (no need for API keys)
- 💸 Runs on "low end" LLM Hardware (demo video uses a 7b model)
- 🤓 Progress logs, allowing for a better understanding of the search process
- 🤔 Follow-up questions
- 📱 Mobile friendly interface
- 🚀 Fast and easy to deploy with Docker Compose
- 🌐 Web interface, allowing for easy access from any device
- 💮 Handcrafted UI with light and dark mode

## Status 

This project is still in its very early days. Expect some bugs. 

## How it works 

Please read [infra](https://github.com/nilsherzig/LLocalSearch/issues/17) to get the most up-to-date idea.

## Self-hosting & Development

### Requirements

- A running [Ollama](https://ollama.com/) server, reachable from the container
    - GPU is not needed, but recommended
- Docker Compose

> [!WARNING]
> Please read [Ollama Setup Guide](./OLLAMA_GUIDE.md) to get Ollama working with LLocalSearch.

### Run the latest release

Recommended, if you don't intend to develop on this project.

```bash
git clone https://github.com/nilsherzig/LLocalSearch.git
cd ./LLocalSearch
# 🔴 check the env vars inside the compose file (and `env-example` file) and change them if needed
docker-compose up 
```

🎉 You should now be able to open the web interface on http://localhost:3000. Nothing else is exposed by default.

### Run the current git version 

Newer features, but potentially less stable.

```bash
git clone https://github.com/nilsherzig/LLocalsearch.git
# 1. make sure to check the env vars inside the `docker-compose.dev.yaml`.
# 2. Make sure you've really checked the dev compose file not the normal one.

# 3. build the containers and start the services
make dev 
# Both front and backend will hot reload on code changes. 
```

If you don't have `make` installed, you can run the commands inside the Makefile manually.

Now you should be able to access the frontend on [http://localhost:3000](http://localhost:3000).

# LLocalSearch

## What it is

This is a completely locally running meta search engine using LLM Agents. The user can ask a question and the system will use a chain of LLMs to find the answer. The user can see the progress of the agents and the final answer. No OpenAI or Google API keys are needed.

Here is a video of it in action, running completely locally (now with darkmode):

https://github.com/nilsherzig/LLocalSearch/assets/72463901/e22a2578-3847-43fa-8317-2af514aa90d7

Now with follow up quesions: 

![image](https://github.com/nilsherzig/LLocalSearch/assets/72463901/9b5bcb39-eb9b-452f-b2c3-b06ecb772e2f)

## Status 

This is a proof of concept, the code is horrible. I didn't intend to make this public yet, but I wanted to share it with a few people.
Please open issues and PRs if you have any suggestions.

## Features 

- Completely local (no need for API keys)
- Runs on "low end" LLM Hardware (demo video uses a 7b model)
- User can see the progress of the agents and understand how the answer was found

## Roadmap 

- Separating "agent updates" / debug information from the final result (something like the [langsmith interface](https://docs.smith.langchain.com/)?)
- Implement a stateful agent chain (so the user can ask follow up questions)
- Code refactoring to provide a more solid base for future development and collaboration

## How it works 

Please read [infra](https://github.com/nilsherzig/LLocalSearch/issues/17) to get the most up to date idea.

## Self-hosting / Development

Currently, both options are the same. I plan to package this into a single docker image for easier deployment.

### Requirements

- A running [Ollama](https://ollama.com/) server somewhere in your network
    - set the `OLLAMA_HOST` environment variable in the `docker-compose.dev.yaml` file to the IP of your Ollama server
    - set the `OLLAMA_MODEL_NAME` environment variable in the `docker-compose.dev.yaml` file to the model name you want to use
        - LLocalSearch is tested with the `hermes-2-pro-mistral` model and will pull this model by default
- Docker Compose
- Make (optional)

Included in the compose file are 
- search backend (based on the Go Langchain library)
- search frontend (svelte & tailwind)
- Chroma DB (for storing search results in vector space)
- SearXNG (meta search engine used by the agent chain)
- Redis (for caching search results)

```bash
git clone https://github.com/nilsherzig/LLocalsearch.git
# make sure to check the env vars inside the compose file
# build the containers and start the services
make dev 
# make dev will start the frontend on port 3000. Both front and backend will hot reload on code changes. 
# or use "make run" to detach the containers (and use "make stop" to stop them)
# running "make upgrade" will stop all containers, pull the latest code and restart the containers
```

If you don't have `make` installed, you can run the commands inside the Makefile manually.

Now you should be able to access the frontend on [http://localhost:3000](http://localhost:3000). All other services are not exposed to the outside world.

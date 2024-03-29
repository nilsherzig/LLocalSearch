# LLocalSearch

## Status 

This is a proof of concept, the code is horrible. I didn't intend to make this public yet, but I wanted to share it with a few people.
Please open issues and PRs if you have any suggestions.

Here is a video in aciton (dont worry a darkmode is underway haha):

https://github.com/nilsherzig/LLocalSearch/assets/72463901/d5ee9232-603e-4471-9e6b-b52ae46f7def

## How it works 

1. The user query is sent to server
2. The server starts an agent chain ("imagine LLMs taking with each other")
3. The agent (in our case `starling-lm` running on Ollama) will process the query and select one of its tools
    - Websearch (using SearXNG) will scrape the top `n` results from the web (the agent will choose the search query). Remove all unnecessary information (like html tags) and store the result into a vector db
    - SearchVectorDB will search the vector db for the most similar results to the query
4. Step 3 will happen multiple times, with the agent choosing different tools and combining the results
5. The final result is sent back to the user and displayed in the frontend

While this is happening, the user can see the progress of the agents steps in the frontend.

## Running / Development

### Requirements

- A running [Ollama](https://ollama.com/) server somewhere in your network
    - a model named `search:latest` 
        - (I recommend `hermes-2-pro-mistral` or `starling-lm-7b-beta`)
    - `all-minilm` for embeddings
- Docker Compose

Included in the compose file are 
- search backend (based on the Go Langchain library)
- search frontend (svelte & tailwind)
- Chroma DB (for storing search results in vector space)
- SearXNG (meta search engine used by the agent chain)
- Redis (for caching search results)

```
git clone https://github.com/nilsherzig/LLocalsearch.git
# make sure to check the env vars inside the compose file
# build the containers and start the services
make dev 
```

## Roadmap

### backend 

- Automatic LLM download to Ollama if not present
- Forced backlinks in final answer
    - I have no idea how to do this, besides asking the LLM haha
        - Force the search LLM to respond in JSON with links?
            - Another round of formatting LLM after this to include these links into the final text?
- Persistent user accounts
    - add your own files 
- select splitter values based on complexity of the question
    - broad vs narrow

### interface

- CLI version
- "tree view" for actions 
- different LLM backends (like OpenAI or Anthropic)
    - settings table for different models
- Ollama model select dropdown 
    - would require a lot of other values to be exposed to the UI
- render latex

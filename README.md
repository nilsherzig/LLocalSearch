# LLocalSearch

LLocalSearch scrapes configured websites, stores cleaned page content locally, and exposes a frontend search server.

## Subcommands

The main entrypoint is `llocalsearch` with three operating modes:

- `llocalsearch scrape -config scraper.example.yaml`
  Scrapes the configured websites and writes cleaned pages directly to `saved_pages` in the SQLite database. This mode does not generate embeddings.
- `llocalsearch embed -config scraper.example.yaml`
  Runs the manual embedding backfill job. It finds all scraped pages that do not yet have an embedding and writes their vectors to `page_embeddings`.
- `llocalsearch web -config scraper.example.yaml`
  Starts the frontend and search server on `:8080` by default. The dashboard shows scrape counts and current embedding coverage.

Typical flow:

```sh
go run ./cmd/llocalsearch scrape -config scraper.example.yaml
go run ./cmd/llocalsearch embed -config scraper.example.yaml
go run ./cmd/llocalsearch web -config scraper.example.yaml
```

`web` and search require a valid `embeddings` configuration because query embeddings are generated at request time. `scrape` can run without any embedding configuration.

## Make Targets

Convenience targets:

- `make scrape`
- `make embed`
- `make web`
- `make start-ollama`

Override defaults if needed:

```sh
make scrape CONFIG=./my-config.yaml
make web CONFIG=./my-config.yaml ADDR=:9090
make web TEMPLATES_DIR=./frontend/templates
```

## Search API

The frontend server now exposes a JSON search endpoint alongside the HTML UI:

- `GET /api/search?q=<query>`
- `GET /api/pages/<id>`

The response contains the normalized query and a list of matching pages ordered by vector similarity.

Example response:

```json
{
  "query": "kubernetes guide",
  "results": [
    {
      "id": 1,
      "url": "https://example.com/article",
      "host": "example.com",
      "path": "/article",
      "scraped_at": "2026-03-23T10:11:12Z",
      "excerpt": "clean article body",
      "similarity": 0.9821
    }
  ]
}
```

Use the `id` from a search result to fetch the full stored page content:

```json
{
  "id": 1,
  "url": "https://example.com/article",
  "host": "example.com",
  "path": "/article",
  "scraped_at": "2026-03-23T10:11:12Z",
  "content": "<article><p>clean article body</p></article>"
}
```

If the server cannot complete the search, it returns `500` with:

```json
{
  "error": "internal server error"
}
```

## curl Examples

Search for pages containing a topic:

```sh
curl "http://localhost:8080/api/search?q=kubernetes+guide"
```

Pretty-print the JSON with `jq`:

```sh
curl -s "http://localhost:8080/api/search?q=local+embeddings" | jq
```

Fetch the full stored content for a specific result:

```sh
curl "http://localhost:8080/api/pages/1"
```

Search with URL-encoded spaces and punctuation:

```sh
curl "http://localhost:8080/api/search?q=sqlite-vec%20cosine%20distance"
```

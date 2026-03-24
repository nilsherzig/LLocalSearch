# Firefox Extension

This directory contains a dependency-free Firefox WebExtension for LLocalSearch.

## What it does

- Watches visited `http` and `https` pages after they finish loading.
- Waits briefly for DOM mutations so JS-rendered pages can settle.
- Uploads the final DOM, title, URL, browser name, and local device ID to:
  - `/api/ingest/browser-pages`

## Local install

1. Start LLocalSearch:
   - `make web`
2. Open Firefox:
   - `about:debugging#/runtime/this-firefox`
3. Click `Load Temporary Add-on...`
4. Select this directory's `manifest.json`
5. Open the extension options page and confirm the server base URL, usually `http://127.0.0.1:8080`

## Manual verification

1. Visit a normal HTML page.
2. Visit a JS-rendered page and wait until content appears.
3. Visit a page behind a login after signing in.
4. Run `make embed` so new pages receive embeddings.
5. Search in the LLocalSearch UI and confirm the captured pages appear with `browser_extension` / `firefox` source metadata.

## Packaging

Use the repo target:

```sh
make firefox-extension
```

That writes `dist/llocalsearch-firefox.zip`.

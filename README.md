# go-sd-webui-cli

StableDiffusion WebUI API client for Golang.

This only implements open API under `/sdapi/*` path, but not the internal HTTP APIs such as `/run/{api}` as they are designed for frontend.

You must run WebUI with `--api` to enable the open API endpoints, see `http://127.0.0.1:7860/docs` for the docs.

TODO: implement important APIs.

TODO: add comments.

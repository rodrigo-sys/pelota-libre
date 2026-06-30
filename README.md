# pelota-libre

Browse live football streams from a terminal. Select a match, pick a channel, watch in your browser or mpv.

## How it works

Scrapes match listings from public websites (librepelota.su, pirlotvplay.dev) and presents them in a Bubble Tea TUI. Stream links open in your browser because the sources use DRM that native players can't handle.

## Install

```sh
go install github.com/rodrigo-sys/pelota@latest
```

Or grab a binary from the [releases page](https://github.com/rodrigo-sys/pelota/releases).

## Usage

```
pelota-go            # default source (pelotalibre)
pelota-go --web      # force browser for all streams
pelota-go --source pirlotv
pelota-go --refresh  # skip cache
```

### Controls

| Key | Context | Action |
|-----|---------|--------|
| `j`/`k` or `↑`/`↓` | any list | navigate |
| `l` or `enter` | match list | show streams |
| `l` or `enter` | stream list | play in mpv/browser |
| `b` | stream list | force browser |
| `h` or `esc` | back / quit |
| `s` | any list | switch source |
| `r` | match list | refresh |
| `q` / `ctrl+c` | any | quit |

## Sources

- **pelotalibre** (default) — scrapes librepelota.su
- **pirlotv** — scrapes pirlotvplay.dev
- **la18hd** — JSON API (currently seized)

## Build from source

```sh
git clone https://github.com/rodrigo-sys/pelota
cd pelota-libre
go build -o pelota .
```

## Legal

This tool does not host, store, or distribute any copyrighted content. It scrapes publicly available HTML pages and displays links. All content is served by third-party websites. Users are responsible for complying with local laws.

The project exists because I wanted a faster way to browse match listings from a terminal. Nothing more.

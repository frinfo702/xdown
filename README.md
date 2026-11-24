# xdown

Download videos (and optionally photos) from X/Twitter and Instagram posts.

## Install

```sh
go install github.com/frinfo702/xdown/cmd/xdown@latest
```

## Usage

```sh
xdown [-d dir] [-p] <url>
```

| Option | Description |
| --- | --- |
| `-d dir` | Output directory (default: current directory) |
| `-p` | Also download photos (default: videos only) |

```sh
xdown https://x.com/user/status/1234567890
xdown -d ~/Downloads https://www.instagram.com/reel/ABC123/
xdown -p 'https://x.com/user/status/1234567890?s=20'
```

Files are saved as `x_<id>_<n>.<ext>` or `ig_<shortcode>_<n>.<ext>`.


## Supported URLs

- X/Twitter: `x.com/<user>/status/<id>`, `twitter.com/<user>/status/<id>`
- Instagram: `instagram.com/p/<code>`, `/reel/<code>`, `/tv/<code>`

## Limitations

- Only public posts are supported.
- Instagram may rate-limit anonymous requests.

# Clear Renovate Notifications

[![CI](https://github.com/wajeht/clear-renovate-notifications/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/wajeht/clear-renovate-notifications/actions/workflows/ci.yml) [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/wajeht/clear-renovate-notifications/blob/main/LICENSE) [![Open Source Love svg1](https://badges.frapsoft.com/os/v1/open-source.svg?v=103)](https://github.com/wajeht/clear-renovate-notifications)

Marks Renovate pull request notifications as done.

## Usage

```bash
GITHUB_TOKEN=github_pat_xxx make run
```

`GITHUB_TOKEN` must be a classic GitHub token with the `notifications` scope.

## Config

- `REPO_FILTER`: repos to check, default `wajeht/*`
- `RENOVATE_BOT_LOGINS`: bot logins to clear, default `wajeht-renovate,wajeht-renovate[bot]`
- `POLL_INTERVAL_SECONDS`: polling delay, default `60`
- `MARK_MODE`: `done` or `read`, default `done`
- `DRY_RUN`: set to `true` to log without clearing

## Development

```bash
make test
make vet
make format
```

## License

Distributed under the MIT License © [wajeht](https://github.com/wajeht). See [LICENSE](./LICENSE) for more information.

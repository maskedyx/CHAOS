<p align="center">
  <a href="#">
    <img src="https://raw.githubusercontent.com/tiagorlampert/CHAOS/master/public/logo.png" alt="CHAOS logo" width="400" height="130">
  </a>
</p>

<h1 align="center">CHAOS: Remote Administration Tool</h1>
<p align="center">
  <a href="https://golang.org/">
    <img src="https://img.shields.io/badge/Golang-1.18+-blue.svg?style=flat-square">
  </a>
    <a href="https://github.com/tiagorlampert/CHAOS/releases">
    <img src="https://img.shields.io/badge/Release-v5 Beta-red.svg?style=flat-square">
  </a>
  <a href="https://github.com/tiagorlampert/CHAOS/blob/master/LICENSE">
    <img src="https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square">
  </a>
  <a href="https://hub.docker.com/r/tiagorlampert/chaos">
    <img src="https://img.shields.io/docker/pulls/tiagorlampert/chaos.svg?style=flat-square">
  </a>
    <a href="https://opensource.org">
    <img src="https://img.shields.io/badge/Open%20Source-%E2%9D%A4-brightgreen.svg?style=flat-square">
  </a>
</p>

<p align="center">
  CHAOS is a free and open-source Remote Administration Tool that allow generate binaries to control remote operating systems.
  <br>
  <a href="https://github.com/tiagorlampert/chaos/issues/new">Report bug</a>
  ·
  <a href="https://github.com/tiagorlampert/chaos/issues/new">Request feature</a>
  ·
  <a href="#quick-start">Quick start</a>
  ·
  <a href="#screenshots">Screenshots</a>
</p>


## Disclaimer

THIS SOFTWARE IS PROVIDED "AS IS" WITHOUT WARRANTY OF ANY KIND. YOU MAY USE THIS SOFTWARE AT YOUR OWN RISK. THE USE IS COMPLETE RESPONSIBILITY OF THE END-USER. THE DEVELOPERS ASSUME NO LIABILITY AND ARE NOT RESPONSIBLE FOR ANY MISUSE OR DAMAGE CAUSED BY THIS PROGRAM.


## Table of contents

- [Features](#features)
- [Antivirus false positives](#antivirus-false-positives)
- [Quick start](#quick-start)
- [Databases](#databases)
- [REST API](#rest-api)
- [Video](#video)
- [Screenshots](#screenshots)
- [Contributing](#contributing)
- [Donate](#donate)
- [Sponsors](#sponsors)
- [Copyright and license](#copyright-and-license)

## Features

| Feature         |  <img src="https://img.shields.io/badge/Windows-0078D6?style=for-the-badge&logo=windows&logoColor=white"/>   |  <img src="https://img.shields.io/badge/Linux-FCC624?style=for-the-badge&logo=linux&logoColor=black"/> | <img src="https://img.shields.io/badge/macOS-000000?style=for-the-badge&logo=apple&logoColor=white"/> |
|:----------------|:-------:|:------:|:------:|
| `Reverse Shell` |    X    |    X   |   X    |
| `Download File` |    X    |    X   |   X    |
| `Upload File`   |    X    |    X   |   X    |
| `Delete File`   |    X    |    X   |   X    |
| `Screenshot`    |    X    |    X   |   X    |
| `File Explorer` |    X    |    X   |   X    |
| `Get OS Info`   |    X    |    X   |   X    |
| `Run Hidden`    |    X    |    X   |   X    |
| `Mask Name`     |         |    X   |   X    |
| `Restart`       |    X    |    X   |   X    |
| `Shutdown`      |    X    |    X   |   X    |
| `Lock screen`   |    X    |        |   X    |
| `Sign out`      |    X    |        |   X    |
| `Open Url`      |    X    |    X   |   X    |

macOS clients can be built for both `amd64` (Intel) and `arm64` (Apple Silicon). Builds are unsigned, so on first run either right-click the binary and choose **Open** or remove the quarantine attribute:

```bash
$ xattr -d com.apple.quarantine <binary>
```

The agent needs Screen Recording permission for screenshots, and Accessibility/Automation permissions for lock, sign out, restart and shutdown (System Settings → Privacy & Security).

Enabling **Run Hidden/Background** builds the client so it starts without a console: on Windows the binary uses the `windowsgui` subsystem, and on Linux/macOS it detaches into its own session with stdout/stderr discarded, returning control to the terminal immediately. When no filename is given, hidden builds are named `systemd` on Linux and `.systemd` on macOS.

**Mask Name** additionally hides the process from process listings on Linux and macOS: it is renamed at runtime and on Linux the command line is cleared, so `ps -ef | grep <name>` and `top` show a blank command. It is not available on Windows, because a process cannot be renamed at runtime there.

Remaining ways to spot a masked client: `/proc/<pid>/exe`, `lsof`, listener sockets, and the file on disk.

## Antivirus false positives

Remote administration tools are routinely flagged by heuristics, so generated clients are hardened before they are compiled:

* the sources are copied and their module path is replaced with a random `github.com/<owner>/<repo>` path, so the well-known CHAOS import paths are not embedded in the binary
* the hidden-mode environment variable is randomized as well
* Windows clients get a version resource, an icon and an application manifest
* the build uses `-trimpath`, no VCS stamp and no build ID, and every client gets a random version string

If a scanner still removes the file, exclude the server working directory (or `temp/`) from real-time protection and rebuild, or whitelist the binary after building. For production use, signing the generated binary with your own code-signing certificate removes most false positives.

## Quick start

Some install options are available:

### 1. Local Development
<details>
  <summary>See more:</summary>

```bash
# Install dependencies
$ sudo apt install golang git -y

# Get this repository
$ git clone https://github.com/tiagorlampert/CHAOS

# Go into the repository
$ cd CHAOS/

# Run
$ PORT=8080 SQLITE_DATABASE=chaos go run cmd/chaos/main.go
```
```powershell
# Run in powershell
$env:PORT="8080"
$env:SQLITE_DATABASE="chaos"
go run cmd/chaos/main.go
```

</details>

### 2. Docker
<details>
  <summary>See more:</summary>

#### Linux
```bash
# Create a shared directory between the host and container
$ mkdir ~/chaos-container

$ docker run -it -v ~/chaos-container:/database/ -v ~/chaos-container:/temp/ \
  -e PORT=8080 -e SQLITE_DATABASE=chaos -p 8080:8080 tiagorlampert/chaos:latest
```

#### Windows
```bash
# Create a shared directory between the host and container
$ md c:\chaos-container

$ docker run -it -v c:/chaos-container:/database/ -v c:/chaos-container:/temp/ -e PORT=8080 -e SQLITE_DATABASE=chaos -p 8080:8080 tiagorlampert/chaos:latest
```

</details>

### 3. Deploy on heroku

Try it now on [Heroku](https://www.heroku.com/) with a single click:

[![Deploy to Heroku](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy)

After running go to http://localhost:8080 and login with the default username: `admin` and password: `admin` and change the password.

## Databases

To connect in a supported database you should set some required environment variables:

* SQLite:
  * `SQLITE_DATABASE` = chaos

* PostgreSQL:
  * `POSTGRES_DATABASE` = chaos
  * `POSTGRES_HOST` = localhost
  * `POSTGRES_PASSWORD` = postgres
  * `POSTGRES_PORT` = 5432
  * `POSTGRES_USER` = postgres
  * `POSTGRES_SSL_MODE` = disable/require //optional

## REST API

A JSON API is exposed under `/api/v1` for automation and external integrations. It requires the `admin` user and a bearer token:

```bash
# Obtain a token
$ curl -s -X POST http://localhost:8080/api/v1/auth/login \
    -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"admin"}'
# {"expire":"...","token":"<JWT>"}

# Use it on every other request
$ curl -s http://localhost:8080/api/v1/devices \
    -H 'Authorization: Bearer <JWT>'
```

Devices are identified by their MAC address (`client_id`). The base64 form returned in `address` is also accepted where a `:id` path parameter is used.

| Method | Endpoint | Description |
|:--|:--|:--|
| `POST` | `/auth/login` | Authenticate and get a bearer token. |
| `GET` | `/health` | Health check. |
| `GET` | `/devices` | List connected devices. |
| `GET` | `/devices/:id` | Get a single device. |
| `POST` | `/devices/:id/commands` | Run a command (`{"command":"...","parameter":"..."}`). |
| `GET` | `/devices/:id/screenshot` | Take a screenshot (PNG). |
| `GET` | `/devices/:id/explorer?path=` | List a remote directory. |
| `POST` | `/devices/:id/open-url` | Open a URL (`{"url":"https://..."}`). |
| `GET` | `/devices/:id/files?path=` | Download a file from the device. |
| `POST` | `/devices/:id/files` | Upload a multipart `file` (optional `path` destination). |
| `DELETE` | `/devices/:id/files?path=` | Delete a file on the device. |
| `GET` | `/targets` | List client build targets and architectures. |
| `POST` | `/clients` | Build a client (`server_address`, `server_port`, `os_target`, `os_arch`, `filename`, `run_hidden`, `mask_name`). |
| `GET` | `/clients/:filename` | Download a built client. |

Examples:

```bash
# Run a shell command
$ curl -s -X POST http://localhost:8080/api/v1/devices/AA:BB:CC:DD:EE:FF/commands \
    -H 'Authorization: Bearer <JWT>' -H 'Content-Type: application/json' \
    -d '{"command":"whoami"}'

# Download a remote file
$ curl -s -OJ http://localhost:8080/api/v1/devices/AA:BB:CC:DD:EE:FF/files?path=/home/user/report.pdf \
    -H 'Authorization: Bearer <JWT>'

# Upload a local file to the device
$ curl -s -X POST http://localhost:8080/api/v1/devices/AA:BB:CC:DD:EE:FF/files \
    -H 'Authorization: Bearer <JWT>' \
    -F file=@./payload.txt -F path=/home/user/payload.txt

# Build a macOS (Apple Silicon) client that runs hidden and masked
$ curl -s -X POST http://localhost:8080/api/v1/clients \
    -H 'Authorization: Bearer <JWT>' -H 'Content-Type: application/json' \
    -d '{"os_target":"darwin","os_arch":"arm64","run_hidden":true,"mask_name":true}'
```

Building clients runs `go build` on the server, so the Go toolchain must be available. CORS is enabled for browser-based callers.

## Screenshots
![generate](public/generate.png)

![devices](public/devices.png)

![shell](public/shell.png)

![explorer](public/explorer.png)

## Contributing
See our contributing guide at [CONTRIBUTING.md](../master/CONTRIBUTING.md).

## Donate
If you enjoyed this project, give me a cup of coffee. :)

[![Donate](https://www.paypalobjects.com/en_US/i/btn/btn_donateCC_LG.gif)](https://www.paypal.com/cgi-bin/webscr?cmd=_donations&business=SG83FSKPKCRJ6&currency_code=USD&source=url)

## Copyright and license

>The [MIT License](https://github.com/tiagorlampert/CHAOS/blob/master/LICENSE)
>
>Copyright (c) 2017, Tiago Rodrigo Lampert
>

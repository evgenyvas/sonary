Sonary
======

![Sonary logo](https://evgenyvas.github.io/logo_full_min.png)

🎵 Sonary Audio Server

Sonary is a fast and lightweight audio streaming server built with Go. The application automatically indexes your music directories and provides a clean web interface for seamless playback.

[Documentation](https://evgenyvas.github.io/sonary)

## 🚀 Quick Start with Podman (WSL2 / Linux)

If you run the application inside a container, you don't need to install any dependencies locally-only Podman is required.

### 1. Prerequisites

Make sure you have the following installed on your machine:

* Podman (version 4.0+)
* podman-compose

Recommended Podman configuration (Rootless mode):

~/.config/containers/containers.conf

```
[containers]
dns_servers = ["8.8.8.8", "8.8.4.4"]
[network]
pasta_options = ["--map-host-loopback=169.254.1.2"]
```

### 2. Preparing the Environment

Before starting the container, ensure your database directory is ready and your configuration files are set up:

   1. Create a directory to persist your SQLite database inside your Linux environment:
   
   ```
   mkdir -p ~/dev/data/sqlite
   ```
   
   2. Verify that a production .env file is present in the project root with the correct settings

### 3. Launching the Application

Network routing, FFmpeg integration, and music directory mounting are handled automatically.

#### Option A: Using podman-compose (Recommended)

Run the following in the project root:

```
podman-compose --podman-build-args="--network=host" build
podman-compose up -d
```

On the first run, `podman-compose will` automatically download base layers, compile frontend assets via Vite, build the static Go binary, install FFmpeg, and launch the server in the background.

#### Option B: Using pure Podman CLI

If you prefer not to use `podman-compose`, build and run the container manually:

```
podman build --network=host -t sonary-prod:latest -f Containerfile .
podman run -d \
  --name sonary-app \
  --restart unless-stopped \
  -p 3101:3101 \
  -v ~/dev/data/sqlite:/data/sqlite:rw,U \
  -v ./var:/app/var:rw,U \
  -v /mnt/d/Music/_audio:/data/music_d:ro \
  -v /mnt/c/Music/_audio:/data/music_c:ro \
  sonary-prod:latest
```

### 4. Usage

Once the container is up and running, open your browser and navigate to:
👉 http://localhost:3101

------------------------------
## 🛠️ Useful Management Commands

* View runtime logs (real-time):

```
podman-compose logs -f
```

* Stop the server:

```
podman-compose down
```

* Force rebuild (after making code changes to backend or frontend):

```
podman-compose up -d --build
```

## 📂 Mounted Volumes Structure

The container interacts with the host machine using the following volumes defined in `compose.yml`:

* ~/dev/data/sqlite - Persistent storage for the SQLite database (write access).
* ./var - Local thumbnail and album art cache (automatically generated inside the project folder).
* /mnt/c/Music/_audio and /mnt/d/Music/_audio - Your music collections (mounted in Read-Only mode to ensure the safety of your personal files).

## 🐧 Native Linux Installation (Without Containers)

### 1. Install Dependencies

```
apt update
apt install ffmpeg
```

### 2. Configure the Environment

Create a configuration file named `.env.local` in the project root. If `.env.local` is not present, default parameters from `.env` will be used. All parameters defined in `.env.local` take priority over other configuration files.

Key parameters to configure:

- APP_ENV - Specifies environment-specific config files, either `dev` or `prod`
- HOST — Specifies the address and port the server will bind to (e.g., :8080)
- ROOT_PATH — Paths to the directories where your music is stored

### 3. Build and Compile

Download Go modules and build the backend binary:

```
go mod download
make build
```

Build the frontend UI:

```
cd ./frontend
yarn install
yarn build
```

### 4. Start the Server

```
./sonary
```

If the `HOST` parameter in your config file equals `:8080`, the web interface will be available at `http://localhost:8080`.

You can also set this up as a permanent system service using the example provided in `support/systemd/sonary.service`.

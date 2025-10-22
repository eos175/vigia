
# 🧭 Vigía

**Vigía** is a lightweight process supervisor and auto-relauncher written in Go.  
It watches a given command, restarts it automatically if it crashes, and ensures graceful shutdown on system signals.  
Think of it as a small guardian that keeps your critical services alive — without the complexity of full process managers like `systemd` or `supervisord`.

---

## ✨ Features

- 🔁 **Automatic restart** when the process exits with an error.  
- ⏱️ **Exponential backoff** between restarts (up to 30s).  
- 🧹 **Graceful shutdown** on `SIGINT` or `SIGTERM`.  
- ⚙️ **Configurable retries** and behavior via flags.  
- 🪶 Minimal overhead — compiled to a single static binary.

---

## 🚀 Usage

```bash
vigia [options] <command> [args...]
````

### Example

```bash
vigia ./my_server --port 8080
```

### Options

| Flag               | Description                                                 | Default |
| ------------------ | ----------------------------------------------------------- | ------- |
| `--always-restart` | Restart even if process exits cleanly                       | `false` |
| `--max-restarts`   | Maximum number of restart attempts before exiting           | `10`    |
---

## 🧠 How It Works

1. Vigía starts the given command as a child process.
2. If the process exits unexpectedly, it logs the error and restarts it.
3. Each restart increases the delay exponentially (1s → 2s → 4s … up to 30s).
4. When Vigía receives a `SIGINT` or `SIGTERM`, it forwards the signal to the child process, waits up to 5 seconds for it to exit gracefully, then terminates.

---

## 📦 Installation

### From source

```bash
go install github.com/eos175/vigia@latest
```

### Manual build

```bash
git clone https://github.com/eos175/vigia.git
cd vigia
go build -ldflags="-s -w" -o vigia .
```

---

## 🧩 Example Output

```
2025-10-13T18:00:00-0600 INF Starting process: ./my_server [--port 8080]
2025-10-13T18:00:10-0600 ERR Process exited with error: exit status 1
2025-10-13T18:00:10-0600 WRN Restarting in 2s (attempt 1/10)
2025-10-13T18:00:12-0600 INF Starting process: ./my_server [--port 8080]
```

---

## ⚖️ License

MIT License © 2025 Emmanuel Ortiz

---

## 🧭 Name Meaning

> “**Vigía**” (Spanish) — *The Watcher*.
>
> In Spanish, a *vigía* is someone who keeps watch from a tower or ship — a perfect metaphor for a program that stands guard over your processes.

---

## 💡 Inspiration

* Simplicity of `supervisord`
* Minimalism of `forever` (Node.js)
* Reliability of `systemd`, without the overhead

---

### 🐧 Example use cases

* Keep your Go service running on a bare-metal server.
* Supervise a Python or Node.js script.
* Auto-restart a CLI tool during testing.

---

```
$ vigia ./server
```

> *“Let Vigía keep watch while you build.”*

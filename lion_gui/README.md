# LION Messenger - Desktop GUI Client

This folder contains the graphical desktop client for **Project LION** (Local Network Messenger). It is built using the **Wails** framework, which links a native Go TCP socket manager to a modern, responsive HTML/CSS/JS frontend.

## Project Structure

```
lion_gui/
  app.go            ← App controller: handles sockets and triggers frontend events
  main.go           ← Entry point: boots up Wails with window options and bindings
  app.md            ← Line-by-line explanation of app.go
  frontend/
    index.html      ← HTML structure for connection and chat screen panels
    package.json    ← Frontend dependencies configuration (Vite, etc.)
    src/
      main.js       ← Handles user actions and listens to background events
      main.md       ← Line-by-line explanation of main.js
      style.css     ← Design tokens and custom scrollbar resets
      app.css       ← Premium glassmorphism design styling & transitions
```

## How It Works

1. **Wails Engine**: Wails compiles the Go backend and bundles the frontend assets into a single desktop binary (`lion_gui.app`).
2. **Go Backend ([app.go](file:///Users/mast/Documents/VInayPrograming/Project_LION/lion_gui/app.go))**:
   - Manages state (connection state, nickname, server address).
   - Starts a background goroutine listener when connected to read TCP data.
   - Leverages `EventsEmit` to notify the frontend when new messages or files are available.
   - Provides methods like `Connect`, `SendMessage`, `SelectAndSendFile`, and `DownloadFile` that are bound to the window context so JavaScript can call them directly.
3. **JS Frontend ([main.js](file:///Users/mast/Documents/VInayPrograming/Project_LION/lion_gui/frontend/src/main.js))**:
   - Updates visual views, appends chat text, and updates loading overlays.
   - Listens to backend status updates (`upload_status`, `download_status`, `message`, `disconnected`).
   - Renders incoming shared files as interactive download cards.

## Live Development

You can run the GUI application in live development mode:

```bash
wails dev
```

This runs a local hot-reloader. Any edits to CSS, JS, HTML, or Go files will update the running app window immediately.

## Building the Standalone App

To build a production binary bundle:

```bash
wails build
```

This generates `build/bin/lion_gui.app` (on macOS) or `build/bin/lion_gui.exe` (on Windows). You can run this file directly to chat on the local network!

# Lyra

Lyra is a terminal-based interactive chatbot. It is built in Go and features a provider-agnostic responder agent harness, allowing you to connect it to local model runners, cloud-based LLM APIs, or even package models directly inside the executable.

---

## Getting Started

### 1. Run Directly
```bash
go run main.go
```

### 2. Build and Run Executable
```bash
go build -o lyra
./lyra
```

---

## Commands
While chatting with Lyra, you can use these special commands:
*   `>>debug`: Bypasses the LLM and prints system status (e.g., placeholder heartrate).
*   `exit` or `quit`: Terminates the interactive session cleanly.

---

## Configuration (`.env`)

Lyra automatically loads environment variables from a local `.env` file at startup. You can configure the chatbot using the following variables:

| Variable | Description | Allowed Values / Examples |
| :--- | :--- | :--- |
| `LYRA_RESPONDER_TYPE` | Selects which responder harness to load. Defaults to `mock` if empty. | `mock`, `gemini`, `openai`, `local-binary`, `embedded` |
| `LYRA_API_KEY` | Authentication key required for cloud LLM APIs. | *Your API Key* (e.g., Cerebras or Gemini key) |
| `LYRA_BASE_URL` | Base API endpoint (for `openai` type). Defaults to Cerebras endpoint if empty. | `https://api.cerebras.ai/v1` (Cerebras), `http://localhost:11434/v1` (Ollama) |
| `LYRA_MODEL` | Model ID/name to query, or the GGUF model path (for `local-binary` type). | `llama3.1-8b`, `gemini-2.5-flash`, `./models/default.gguf` |
| `LYRA_LOCAL_BINARY_PATH` | Path to the local CLI model runner (for `local-binary` or `embedded` type). | `llama-cli`, `./llamafile-0.8.18` |
| `LYRA_SYSTEM_INSTRUCTION` | The system prompt to govern the behavior and personality of the responder. | `"You are Lyra, a friendly and helpful AI chatbot."` |

---

## Responder Types in Detail

### 1. Mock (`mock`)
The default fallback. It runs entirely offline, requires no configuration or API keys, and echoes your input with the configured system instruction.

### 2. OpenAI-Compatible (`openai`)
Connects to any endpoint supporting the OpenAI Chat Completions API (such as Cerebras, local Ollama, LM Studio, or OpenAI itself). 
Example `.env`:
```env
LYRA_RESPONDER_TYPE=openai
LYRA_API_KEY=csk-your-cerebras-key-here
LYRA_BASE_URL=https://api.cerebras.ai/v1
LYRA_MODEL=llama3.1-8b
```

### 3. Google Gemini (`gemini`)
Connects to Google GenAI API endpoint.
Example `.env`:
```env
LYRA_RESPONDER_TYPE=gemini
LYRA_API_KEY=AIzaSy...your-gemini-key
LYRA_MODEL=gemini-2.5-flash
```

### 4. Local Binary (`local-binary`)
Runs local GGUF models on your machine's CPU/GPU by executing a local command-line tool (such as `llama-cli` or `llamafile`) as a subprocess. This provides native performance with **zero Cgo dependencies**.
Example `.env`:
```env
LYRA_RESPONDER_TYPE=local-binary
LYRA_LOCAL_BINARY_PATH=llama-cli
LYRA_MODEL=./models/default.gguf
```

### 5. Embedded (`embedded`)
Packages a GGUF model directly inside the executable using Go's `//go:embed` directive. At runtime, the model is extracted to a temp folder and executed via the local binary runner. 
*Note: To use this, you must place your model at `responder/models/default.gguf` and re-compile.*

---

## Project Structure
*   [main.go](file:///Users/pratheeksha/lyra/main.go): The application entry point.
*   [interface/](file:///Users/pratheeksha/lyra/interface): Houses the interactive CLI/terminal chat loop.
*   [responder/](file:///Users/pratheeksha/lyra/responder): Contains the provider-agnostic responder harness and concrete implementations (`gemini`, `openai`, `local_binary`, `embedded`, `mock`).

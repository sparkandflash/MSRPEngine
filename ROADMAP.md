# MSRPEngine Roadmap

This document outlines the evolutionary steps for the Mind State Reactive Personality Engine (MSRPEngine). Development is broken down into distinct, focused versions aimed at building a solid framework for experimenting with persistent cognition.

## Immediate Interface Goals
*   **Discord Bot Integration:** Exposing the engine as an active participant in Discord servers.
*   **Browser-Based Web App:** A rich HTML/CSS/JS interface running locally or deployed to the web.

---

## V2.1: The Weaver (Neuromorphic Memory Graph)
**Goal: Shift from flat episodic text to a dynamically linking memory graph.**

*   **The Neuron (Node):** Replaces flat JSON episodes. A Node contains a Fact, an Embedding, Creation Time, and an Access Count.
*   **The JIT Weaver (Spreading Activation):** A deterministic process that triggers during user interaction or self-talk. It uses local embeddings to search the graph and form temporary `ProtoNodes` between related memories for exactly zero API cost.
*   **Validation Loop (Organic Crystallization):** ProtoNodes are injected into the active LLM context. If the LLM uses the connection, the ProtoNode is upgraded to a permanent **Linked-Node** containing a synthesized relationship fact. If unused, it evaporates.

---

## V2.2: The Dreamer (Model Formation & Idle Summarization)
**Goal: Move from storing facts to building abstract beliefs.**

*   **The Idle Summariser (Shortcut Neurons):** A background process that scans long chains of frequently accessed Linked-Nodes (e.g., A -> B -> C -> D) and spends API tokens to compress them into a high-level abstraction (Shortcut Node: A -> D) with its own embedding.
*   **Candidate Models:** Shortcut Neurons act as behavioral hypotheses about the user (e.g., "User values simplicity").
*   **The Soul Score:** Gating trait retention based on the intensity of Reactor mindstate spikes, ensuring Lyra's personality core compounds and evolves organically.

---

## V2.3: The Asynchronous Mind (Async Interface)
**Goal: Break the Chatbot Paradigm.**

*   **Decoupled Request/Response:** The engine separates user input from LLM output. The user drops messages into an environment, and the engine replies at its own biological pace.
*   **The Mode Switch (Convergence vs. Wandering):** When a user message is queued, the Weaver actively connects short-term input to long-term memory. When alone, the passive Weaver naturally wanders the graph.
*   **Energy Economics:** To survive free-tier APIs, loop frequency is modulated by Reactor scores (e.g., Cortisol spikes speed up thoughts). A strict Mental Energy economy acts as a circuit breaker against runaway token usage.

---

## V3: Goja (Cognitive DSL & Skill Library)
**Goal: Solve the Symbol Grounding problem and LLM math failures natively.**

*   **Embedded JavaScript VM:** Integrating `goja` directly into the Go binary to run a native JS environment isolated from the host machine.
*   **The Cognitive DSL:** Allowing the LLM to write tiny, deterministic JavaScript programs (e.g., counters, adders, logical gates) to solve math and strict logic problems rather than guessing probabilistically.
*   **The Skill Library:** When the LLM successfully writes a script that solves a problem, the script is saved as a permanent Node in her memory graph.
*   **Autonomous Web Browsing:** Exposing a `fetch()` wrapper to the Goja VM so the engine can autonomously query the Wikipedia API or read website data during its Wandering Mind/Dreaming states to learn about the world while the user sleeps.

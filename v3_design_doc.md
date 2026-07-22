# V3 Design Document: The Asynchronous Mind

## Paradigm Shift: "The Fly and the Human"
V3 fundamentally shifts the engine from a traditional "request -> response" chatbot into a **Persistent Simulated Mind**. 
To survive on free-tier APIs and accurately simulate cognition, the engine operates on a deliberately slower "frame rate" than human users. If Lyra is the human, the user is a fly. This means delayed responses are not a bug, but an accepted and expected feature. The user simply drops inputs into her environment, and she processes them at her own biological pace.

## The Continuous Thought Cycle
Instead of waiting for user input, the engine runs a continuous asynchronous loop:
1. **Context Formulation:** The current state of mind and immediate context is gathered.
2. **LLM Processing:** The LLM analyzes the context. It can choose to "answer" the context, autocomplete a thought, or ask an internal question.
3. **Context Crawler (Deterministic):** Based on the LLM's output, a deterministic crawler activates. It semantically searches the vector database using the LLM's *current internal thought state* as the query.
4. **Memory Injection & Rotation:** The crawler travels along memory links and timelines, capturing relevant material. This new material is injected back into the LLM's context window. To prevent bloat, old context is aggressively rotated out (FIFO) and replaced by the newly retrieved memory.
5. **Repeat:** The LLM processes the newly injected memory, continuing the chain of thought.

## Asynchronous User Interaction
The engine is always listening to the interface. The user can insert messages into the context at any time. However:
- The LLM does not immediately respond.
- The user's input simply becomes another piece of context for the Crawler and LLM to process.
- A combination of Reactor scores, the Rule Engine, and the LLM's internal state determines *if* and *when* to finally send a response to the user.

## Biological Frequency Control (The Reactor)
The frequency of the thought cycle is not static. It is dynamically controlled by the Reactor's biological mind scores:
- **Minimum Duration:** The absolute minimum time between API calls (e.g., 8 seconds) is set via environmental variables to respect rate limits. (This can be decreased later if rate limits allow).
- **Modulation:** If the engine experiences high Cortisol (stress/anxiety), the loop frequency spikes to simulate racing thoughts (e.g., hitting the 8-second minimum). If she is calm, the mind scores scale the delay up (e.g., 20+ seconds) for deep, slow contemplation.
- **Fluctuation:** One minute might see 3 API calls, while the next minute might see 6, creating a highly organic processing rhythm.

## Resting Periods & Idle Methods
The thought cycle does not run endlessly. The engine utilizes scheduled "Resting Periods" (Hibernation/True Sleep).
During these rests, the high-frequency thought cycle pauses, and heavy **Idle Methods** take over. These deterministic background processes are responsible for:
- Consolidating memories.
- Building, testing, and breaking Candidate Models (Hypotheses about the user and the world).
- Performing semantic linking across the memory graph. 

# AI Agents in Go

**Source:** [Eino — Orchestration Design Principles](https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/)
+ [Eino ADK 0.1](https://www.cloudwego.io/docs/eino/overview/eino_adk0_1/) (primary,
natively Go). Tool-design discipline cross-references
[mcp-server.md](mcp-server.md) (Phil Schmid).

**Applies to:** LLM-driven agents that call tools and orchestrate multi-step
reasoning (ReAct loops, multi-agent topologies). For servers *exposing* tools
to LLMs over MCP see [mcp-server.md](mcp-server.md); for general backend
services see [architecture.md](architecture.md).

This reference wins over generic agent-framework guidance for agent-loop
structure, multi-agent topology, typed data flow between nodes, and the
state-externalism rule below.

> **Framework-agnostic by intent — and here is how to read it.** Every rule
> below is a principle that applies in any framework and in hand-written code.
> Where a principle is clearer with a concrete API, the example comes from
> [Eino](https://github.com/cloudwego/eino) (the donor these principles were
> distilled from) and is fenced off under an **`Eino spelling`** label or an
> inline `eino:` marker.
>
> **Anything so marked is an illustration, not a requirement.** No identifier
> from it needs to exist in your stack; if your framework spells the same idea
> differently, that is fine. Eino is cited as the source of the principles, not
> endorsed as the recommended framework.

## Contents

- [Type-aligned composition (flagship)](#type-aligned-composition-flagship)
- [Read-only input contract](#read-only-input-contract)
- [State-externalism](#state-externalism)
- [The agent loop — ReAct](#the-agent-loop-react)
- [Multi-agent topology — decision tree](#multi-agent-topology-decision-tree)
- [Agent interface — what is actually required](#agent-interface-what-is-actually-required)
- [Delegation primitives](#delegation-primitives)
- [Interrupt / Resume](#interrupt-resume)
- [Components implement only the paradigms real to them](#components-implement-only-the-paradigms-real-to-them)
- [Concurrency hygiene (cross-ref)](#concurrency-hygiene-cross-ref)
- [What to avoid](#what-to-avoid)
- [Sources](#sources)

---

## Type-aligned composition (flagship)

**Reject `map[string]any` as the universal data-passing contract between
nodes.** This is LangChain's central design choice and the one Eino
deliberately rejects:

> "Adopting the generalization to `any` … developers need to explicitly convert
> to specific types when writing code. This would greatly increase developers'
> cognitive burden, so this approach was ultimately abandoned."

The rules:

- **Each node's input/output types stay as the developer expects.** Enforce
  upstream/downstream type alignment at **compile time** (when the graph or
  chain is assembled), not at runtime.
- **The composition primitives are generic in their input and output types**, so
  a mismatch between two wired nodes is a build-time error rather than a
  production panic.
- **`map[string]any` only at convergence boundaries, and only produced by the
  framework** — where a node genuinely must merge several upstreams, the
  framework wraps a value under a key at that boundary and extracts it on the
  other side. Nodes never natively emit or accept maps.
- **Valid type-alignment relations** are a closed set: (1) same type;
  (2) downstream interface + upstream implements it (incl. `any` as trivial
  case); (3) upstream interface + downstream concrete type (only checkable at
  runtime).

> **Eino spelling.** The generic primitives are `Graph[I, O]`, `Chain[I, O]` and
> `StreamReader[T]`; mismatches surface at `Compile()` / `AddXXXNode()`. The
> convergence helpers are `WithOutputKey("k")` (wraps a node's output into a map
> entry) and `WithInputKey("k")` (extracts one field, rather than forcing
> downstream nodes to accept maps).

> This rule is the reason cross-agent state is typed via a generic `S` (see
> State-externalism), never `map[string]any`.

---

## Read-only input contract

**Nodes, branches, and handlers must not mutate their Input.**

Inputs are passed by assignment, not copy — mutating a reference-type Input
causes side effects and **concurrency hazards**, because agent runtimes execute
nodes concurrently: several nodes running in the same execution round can hold a
reference to the same Input value. If modification is needed, **copy first**.
The rule applies equally to chunks of a streamed input.

This is stronger than the usual "don't mutate args": in ordinary code the
aliasing is usually visible in the call graph, whereas here the runtime decides
what runs together, so the sharing is invisible at the call site.

> **Eino spelling.** Eino calls one round of concurrent node execution a
> **SuperStep** — the term comes from Pregel, the graph-execution model its
> engine follows. Streamed chunks arrive as `StreamReader[T]`.

---

## State-externalism

**Keep nodes state-agnostic.** State lives outside the node and reaches it
through typed handlers, rather than being baked into the node.

- **Global state via typed pre/post handlers.** A handler runs before or after
  the node, sits outside it, and affects it only by replacing Input or Output —
  so the node itself never reads or writes shared state directly. Streaming
  variants exist for streamed inputs and outputs.
- **Cross-agent state is a typed value, not `map[string]any`.** Parameterize it
  (a generic `S`) so the compiler checks every access.
- **`map[string]any` is permitted only as a KV-boundary escape hatch** for
  genuinely dynamic data (e.g. a free-form metadata bag), never as the
  universal agent-state contract.
- **In-node state access needs an explicit, locked escape hatch.** When a node
  genuinely must touch shared state, it goes through a function that takes a
  callback and holds a lock across the read/write, rather than reaching for the
  state directly.

> **Eino spelling.** The handlers are `StatePreHandler[S]` /
> `StatePostHandler[S]` plus stream variants; the escape hatch is
> `ProcessState[S any](ctx, handler func(ctx, S) error) error`, and the framework
> locks at all read/write positions.
>
> The typed-state rule also resolves a tension *inside* the Eino docs: the ADK
> document's Shared Session **is** a `map[string]any`, while the orchestration
> document argues for typed state. This skill sides with typed state.

---

## The agent loop — ReAct

**The canonical agent loop is ReAct (Reason → Act → Observe):**

1. **Reason** — call the LLM with the current input + history.
2. **Act** — the LLM returns a tool-call request; execute the tool.
3. **Observe** — return the tool result to the LLM as an observation.
4. Repeat until the LLM emits no tool call (final answer).

**Cap iterations.** An explicit maximum-iteration bound is mandatory — an
unbounded ReAct loop is a bug, not a configuration choice: a model that keeps
emitting tool calls will burn budget until something else stops it. Set the
bound yourself rather than inheriting a default, and check what your
framework's zero value means: in several of them `0` is "unbounded", so
leaving the field unset is the one setting you must never ship.

---

## Multi-agent topology — decision tree

Six named archetypes, taken from the Eino ADK taxonomy — the *shapes* are
general and appear in every multi-agent framework, only the names and the
API differ. Pick by the collaboration shape, not by fashion.

| Topology | When to use | Key property |
|---|---|---|
| **Sequential** | Strict linear pipeline; each agent sees the full input + outputs of preceding agents. | Early-exit on any sub-agent emitting an exit/interrupt action. |
| **Parallel** | Independent sub-tasks over the same input; order doesn't matter. | Sub-agents run in concurrent goroutines, share input, aggregate via `sync.WaitGroup`, output in **order received**. |
| **Loop** | Refinement / accumulation; later iterations access prior history. | Bounded by an iteration cap or an explicit exit signal from a sub-agent (eino: `ExitAction`). |
| **Supervisor** | One router allocates tasks and aggregates; sub-agents are independently developable/testable/replaceable. | Sub-agents return control automatically on completion ("deterministic callback"). |
| **Plan-Execute** | Closed-loop "think → act → adjust": Planner (structured plan) → Executor (first step) → Replanner (continue/replan/end). | Three roles; replanning is explicit. |
| **DeepAgents** | Decompose a large goal into tracked todos, delegate subtasks through a single unified delegation tool (eino: `TaskTool`), isolate sub-agent contexts, main agent aggregates. | Tradeoff: over-decomposition increases call counts and cost. |

---

## Agent interface — what is actually required

Two properties are required of any agent that another agent can invoke. The
**signature is your framework's business**; these two are not.

**1. Identity for discovery.** An agent exposes a name and a description that
are readable at runtime, so a router or supervisor can select it without
hard-coded knowledge. A description written for a human reader and never read by
the router is not identity — see [Delegation primitives](#delegation-primitives).

**2. Event-driven, non-blocking return.** The run entrypoint yields a *stream of
events* — node outputs (replies, tool results), state-modification actions, and
the running trajectory — rather than blocking until a single final value.

**Event-driven return is mandatory**, and for two concrete reasons rather than
as an aesthetic preference:

- **A blocking call hides the trajectory.** Everything the agent did on the way
  to its answer is unobservable, which makes debugging and cost attribution
  guesswork.
- **A blocking call forecloses streaming UX.** You cannot show partial output
  you never received.

Flow control (interrupt, jump, exit) then belongs to the runner that consumes
the event stream — not to the agent, and not to the caller.

> **Eino spelling.** Eino ADK's `Agent` interface is one realization of the two
> properties above:
>
> ```go
> type Agent interface {
>     Name(ctx context.Context) string
>     Description(ctx context.Context) string
>     Run(ctx context.Context, input *AgentInput, options ...AgentRunOption) *AsyncIterator[*AgentEvent]
> }
> ```
>
> `AgentInput`, `AgentRunOption` and `AsyncIterator[*AgentEvent]` are Eino types.
> **Do not implement this interface in a project that does not use Eino** — an
> `iter.Seq2`, a channel of events, or a callback all satisfy the requirement
> equally well.

---

## Delegation primitives

Two ways one agent invokes another — pick by what the callee needs. (The names
are Eino ADK's; the distinction exists in every multi-agent framework, sometimes
unnamed.)

- **Transfer (parent → child)** — carries the full running context. Use when
  the callee needs the conversation/state so far.
- **Agent-as-Tool** — the callee is invoked with only parameters (no full
  context), like any other tool. Use when only the arguments matter.

Choosing wrong (e.g. Agent-as-Tool when the callee actually needs history) is a
common bug — the callee silently lacks context it expected. The failure is quiet
by construction: the callee still returns *an* answer, just one computed without
the history it was designed around, so nothing errors and the quality drop looks
like a model problem.

---

## Interrupt / Resume

Long-running, pausable, or human-in-the-loop scenarios use **Interrupt +
Resume**, with persistence decoupled from resume logic:

1. The agent emits an event carrying an **interrupt** with a state snapshot.
2. The runner hands that snapshot to a **pluggable store** — an interface you
   implement over whatever medium fits (DB, file, Redis).
3. Later, a **resume entrypoint** restarts from the snapshot, identified by a
   checkpoint id, with new input.

**Keep persistence behind a small, single-purpose interface.** Do not bake
serialization or storage into the agent itself: the agent should not know
whether its checkpoint went to Postgres or to a temp file, and a checkpoint
store that grows agent-specific methods has stopped being a store.

> **Eino spelling.** The store interface is `CheckPointStore`; the resume
> entrypoint is `runner.Resume(ctx, checkpointID, newInput)`; the interrupt
> travels as an Interrupt action inside an `Event`.

---

## Components implement only the paradigms real to them

This is the most framework-shaped section in this reference; the transferable
part is the rule, not the taxonomy.

A component should implement only the streaming paradigms that are **real in its
business scenario**, and the orchestrator should adapt between paradigms for the
rest. A chat model that has no meaningful "collect a stream into one value"
behavior should not be forced to write one — an implementation that exists only
to satisfy an interface is a stub that will be wrong the first time someone
relies on it.

The four paradigms are a useful general vocabulary, whatever your framework
calls them: **invoke** (value in, value out), **stream** (value in, stream out),
**collect** (stream in, value out), **transform** (stream in, stream out). If a
framework can derive the missing three from the one you implemented — by
concatenating a stream, wrapping a value, merging, or copying — let it.

> **Eino spelling.** The paradigms are `Invoke` / `Stream` / `Collect` /
> `Transform`; the automatic derivations are Auto Concatenate / Box / Merge /
> Copy. For custom chunk types outside the built-in set (Message, string,
> `[]*Message`, Map, slices), register a concat func with
> `RegisterStreamChunkConcatFunc`.

---

## Concurrency hygiene (cross-ref)

Agent code is concurrent code. Apply [engineering-policy.md](engineering-policy.md)
§Concurrency without exception:

- Every goroutine has an exit condition (`context.Context`, `sync.WaitGroup`,
  or done channel). **No fire-and-forget goroutines in agent loops.**
- `errgroup` for parallel fan-out that should fail-fast.
- Bound concurrency (semaphore / worker pool) when an agent fans out to many
  tools or sub-agents — LLM APIs rate-limit.
- Propagate `ctx` everywhere; honor cancellation promptly.
- Always test with `-race`.

---

## What to avoid

- **`map[string]any` between nodes** — the central anti-pattern this reference
  exists to prevent. Use typed generics.
- **Mutating a shared Input** from concurrently executing nodes — concurrency
  hazards.
- **Unbounded ReAct loops** — set an explicit iteration cap, and never ship the
  framework's zero value without checking whether it means "unbounded".
- **Fire-and-forget goroutines** in agent loops — every goroutine needs an exit
  condition.
- **Agent-as-Tool when the callee needs full context** — silently produces
  broken delegation.
- **Baking serialization/storage into the agent** — put it behind a small,
  pluggable checkpoint-store interface.
- **Forcing every component to implement every streaming paradigm** — let the
  orchestrator adapt between them.
- **Implementing a donor framework's interface verbatim in a project that does
  not use that framework** — everything marked `Eino spelling` above is an
  illustration of a principle, not an API to copy.

---

## Sources

- [Eino — Orchestration Design Principles](https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/) —
  type-aligned composition, read-only input contract, state-externalism,
  component paradigm rule. ~70% transferable; the rest is Eino-specific
  plumbing (Pregel/DAG engines) not adopted here.
- [Eino ADK 0.1](https://www.cloudwego.io/docs/eino/overview/eino_adk0_1/) —
  ReAct loop, six topology archetypes, unified Agent interface, Interrupt/
  Resume, delegation primitives. ~40% transferable; the rest is ADK API
  surface, cited as patterns not prescriptions.
- [Phil Schmid — MCP best practices](https://www.philschmid.de/mcp-best-practices)
  (via [mcp-server.md](mcp-server.md)) — tool-design discipline that applies
  equally to agent tool-calling.

**On the `Eino spelling` blocks.** Eino supplies this reference's worked examples
because it is the donor the principles were distilled from, not because it is
recommended. Each block is quarantined so a reader on Google ADK, LangChainGo or
hand-written code can skip it without losing a rule: every mandatory statement in
this file is outside those blocks. If you add material here from another
framework, keep the same split — principle in the body, API in a marked block.

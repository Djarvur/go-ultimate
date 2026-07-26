# AI Agents in Go

**Source:** [Eino — Orchestration Design Principles](https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/)
+ [Eino ADK 0.1](https://www.cloudwego.io/docs/eino/overview/eino_adk0_1/) (primary,
natively Go). Tool-design discipline cross-references
[mcp-server.md](mcp-server.md) (Phil Schmid). Full donor analysis in
[`../../skills-analysis.md`](../../skills-analysis.md) §7.

**Applies to:** LLM-driven agents that call tools and orchestrate multi-step
reasoning (ReAct loops, multi-agent topologies). For servers *exposing* tools
to LLMs over MCP see [mcp-server.md](mcp-server.md); for general backend
services see [architecture.md](architecture.md).

This reference wins over generic agent-framework guidance for agent-loop
structure, multi-agent topology, typed data flow between nodes, and the
state-externalism rule below.

> This reference is **framework-agnostic by intent**. Eino is cited as the
> source of principles, not endorsed as the recommended framework. Apply the
> rules in whatever framework (or hand-written code) you use.

## Contents

- [Type-aligned composition (flagship)](#type-aligned-composition-flagship)
- [Read-only input contract](#read-only-input-contract)
- [State-externalism](#state-externalism)
- [The agent loop — ReAct](#the-agent-loop-react)
- [Multi-agent topology — decision tree](#multi-agent-topology-decision-tree)
- [Unified Agent interface shape](#unified-agent-interface-shape)
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
  upstream/downstream type alignment at **compile time** (when the graph/chain
  is built), not at runtime.
- **Generics at boundaries** — `Graph[I, O]`, `Chain[I, O]`,
  `StreamReader[T]`. Type mismatches surface at `Compile()` / `AddXXXNode()`,
  not in production.
- **`map[string]any` only at convergence boundaries**, produced by the
  framework (e.g. `WithOutputKey("k")` wraps a node's output into a map entry),
  never natively emitted by nodes. Symmetrically, `WithInputKey("k")` extracts
  one field rather than forcing downstream nodes to accept maps.
- **Valid type-alignment relations** are a closed set: (1) same type;
  (2) downstream interface + upstream implements it (incl. `any` as trivial
  case); (3) upstream interface + downstream concrete type (only checkable at
  runtime).

> This rule directly informs **conflict #17** in the analysis: cross-agent
> state is typed via a generic `S` (see State-externalism), not `map[string]any`.

---

## Read-only input contract

**Nodes, branches, and handlers must not mutate their Input.**

Inputs are passed by assignment, not copy — mutating a reference-type Input
causes side effects and **concurrency hazards** (multiple SuperSteps may touch
the same value). If modification is needed, **copy first**. The rule applies
equally to `StreamReader` chunks.

This is stronger than the usual "don't mutate args" because agents routinely
execute nodes concurrently across SuperSteps.

---

## State-externalism

**Keep nodes state-agnostic.** State flows through external handlers, not
baked into nodes.

- **Global state via typed Pre/Post handlers** (`StatePreHandler[S]` /
  `StatePostHandler[S]`, plus stream variants). They sit outside the node and
  affect it only by replacing Input/Output.
- **Cross-agent state is typed via a generic `S`** — never `map[string]any`.
  This resolves the internal Eino tension (Doc 2's Shared Session *is*
  `map[string]any`; we side with Doc 1's typed-state thesis).
- **`map[string]any` is permitted only as a KV-boundary escape hatch** for
  genuinely dynamic data (e.g. a free-form metadata bag), never as the
  universal agent-state contract.
- **In-node state access** via the explicit `ProcessState[S any](ctx, handler
  func(ctx, S) error) error` escape hatch when a node genuinely needs it. The
  framework locks at all read/write positions.

---

## The agent loop — ReAct

**The canonical agent loop is ReAct (Reason → Act → Observe):**

1. **Reason** — call the LLM with the current input + history.
2. **Act** — the LLM returns a tool-call request; execute the tool.
3. **Observe** — return the tool result to the LLM as an observation.
4. Repeat until the LLM emits no tool call (final answer).

**Cap iterations.** `MaxIterations` is mandatory — an unbounded ReAct loop is a
bug. `MaxIterations = 0` may mean "unbounded" in some frameworks; treat that
as a smell and set an explicit bound.

---

## Multi-agent topology — decision tree

Six named archetypes (Eino ADK). Pick by the collaboration shape, not by
fashion.

| Topology | When to use | Key property |
|---|---|---|
| **Sequential** | Strict linear pipeline; each agent sees the full input + outputs of preceding agents. | Early-exit on any sub-agent emitting an exit/interrupt action. |
| **Parallel** | Independent sub-tasks over the same input; order doesn't matter. | Sub-agents run in concurrent goroutines, share input, aggregate via `sync.WaitGroup`, output in **order received**. |
| **Loop** | Refinement / accumulation; later iterations access prior history. | Bounded by `MaxIterations` or an `ExitAction`. |
| **Supervisor** | One router allocates tasks and aggregates; sub-agents are independently developable/testable/replaceable. | Sub-agents return control automatically on completion ("deterministic callback"). |
| **Plan-Execute** | Closed-loop "think → act → adjust": Planner (structured plan) → Executor (first step) → Replanner (continue/replan/end). | Three roles; replanning is explicit. |
| **DeepAgents** | Decompose a large goal into tracked todos, delegate subtasks via one unified `TaskTool`, isolate sub-agent contexts, main agent aggregates. | Tradeoff: over-decomposition increases call counts and cost. |

---

## Unified Agent interface shape

Every agent implements:

```go
type Agent interface {
    Name(ctx context.Context) string
    Description(ctx context.Context) string
    Run(ctx context.Context, input *AgentInput, options ...AgentRunOption) *AsyncIterator[*AgentEvent]
}
```

- `Name` and `Description` give the agent a clear identity for discovery and
  invocation by other agents.
- `Run` returns an **`*AsyncIterator[*AgentEvent]`**, not a blocking value.
  Events carry node outputs (replies, tool results), state-modification
  actions, and the running trajectory.

**Event-driven return is mandatory.** Blocking calls hide the trajectory and
block streaming UX. Flow control (interrupt, jump, exit) is handled
automatically by the runner framework.

---

## Delegation primitives

Two ways one agent invokes another — pick by what the callee needs:

- **Transfer (parent → child)** — carries the full running context. Use when
  the callee needs the conversation/state so far.
- **Agent-as-Tool** — the callee is invoked with only parameters (no full
  context), like any other tool. Use when only the arguments matter.

Choosing wrong (e.g. Agent-as-Tool when the callee actually needs history) is a
common bug — the callee silently lacks context it expected.

---

## Interrupt / Resume

Long-running, pausable, or human-in-the-loop scenarios use **Interrupt +
Resume**, with persistence decoupled from resume logic:

1. The agent emits an `Event` containing an **Interrupt action** (carrying a
   state snapshot).
2. The runner persists state via a registered **`CheckPointStore`** interface
   (you implement the storage medium — DB, file, Redis).
3. Later, `runner.Resume(ctx, checkpointID, newInput)` restarts from the
   breakpoint with the new input.

**Implement `CheckPointStore` as a small, single-purpose interface.** Do not
bake serialization/storage into the agent itself.

---

## Components implement only the paradigms real to them

A `ChatModel` does not need to implement `Collect`. Components implement only
the streaming paradigms (Invoke / Stream / Collect / Transform) that are real
in their business scenario; the orchestrator auto-fills the missing ones (Auto
Concatenate / Box / Merge / Copy).

For custom chunk types not in the built-in set (Message, string, `[]*Message`,
Map, slices), register a concat func via the framework's equivalent of
`RegisterStreamChunkConcatFunc`.

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
- **Mutating shared Input** across nodes / SuperSteps — concurrency hazards.
- **Unbounded ReAct loops** — set an explicit `MaxIterations`.
- **Fire-and-forget goroutines** in agent loops — every goroutine needs an exit
  condition.
- **Agent-as-Tool when the callee needs full context** — silently produces
  broken delegation.
- **Baking serialization/storage into the agent** — use a separate
  `CheckPointStore`.
- **Forcing every component to implement every streaming paradigm** — let the
  orchestrator auto-fill.

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

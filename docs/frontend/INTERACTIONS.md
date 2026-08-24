# Agent-native interaction rules

## Run lifecycle

Show an understandable execution sequence when it adds value:

```text
User message
  -> Agent status
  -> Tool request
  -> Approval, when required
  -> Tool result
  -> Agent response
```

Do not expose raw chain-of-thought, model-provider payloads, stack traces, or protocol envelopes in the default Conversation view. Provide concise status plus optional technical details such as run ID, timing, tool arguments/result summary, and error code in an inspector or expandable detail.

Canonical user-facing Run states:

- **Queued**: accepted but not started.
- **Running**: model or orchestration is active.
- **Using tool**: a named tool is pending or running.
- **Approval required**: execution is paused until the user decides.
- **Completed**: durable assistant output is available.
- **Failed**: preserve partial context and offer an appropriate retry/details action.
- **Cancelled**: make the stopped state explicit; never present it as a generic failure.
- **Reconnecting**: execution may continue on the server while events are restored.

Status transitions must come from durable Run/event state when correctness matters. Optimistic UI may improve immediacy but must reconcile with the server.

## Tool execution

Tool UI uses assistant-ui Tool UI rendering and AG-UI `TOOL_CALL_*` events. The Conversation view shows a compact expandable tool card; the optional Run Inspector shows the current Run ID, terminal state, ordered tool calls, arguments, and results. Raw arguments/results remain secondary diagnostic details rather than the primary answer.

| State               | UI behavior                                                                                        |
| ------------------- | -------------------------------------------------------------------------------------------------- |
| Pending             | Show tool name and intended action; avoid indefinite generic dots                                  |
| Running             | Show active state, cancellability when supported, and elapsed time only if useful                  |
| Succeeded           | Show a compact result summary with expandable details; allow relevant follow-up action             |
| Failed              | Preserve tool name/input context, explain the recoverable cause, and offer retry/details when safe |
| Permission required | Explain which capability or resource is missing and how to grant it                                |
| Approval required   | Show exact external effect, target, scope, and editable parameters when applicable                 |

Do not render raw JSON as the primary successful result. Raw input/output may be available behind `View details` for diagnostics, with secrets redacted by the server.

### User-specified Tool

- Typing `@` and pressing the Composer `Tools` button open the same assistant-ui Mention Catalog.
- The first catalog category is `Tools`; entries show Tool name, MCP Plugin, description, and availability.
- Only one Tool can be selected. A later selection replaces the earlier selection and appears as a removable Composer chip; no hidden directive text is added to the user message.
- `needs-agent-enable` and `tool-disabled` entries perform an explicit Agent binding mutation before selection. Offline and approval-required entries explain why they cannot run.
- A failed enable action preserves the message draft and provides retry. A selected Tool is cleared only after `RUN_STARTED`, not when Send is pressed.
- The Run Inspector distinguishes the user-specified Tool from actual `TOOL_CALL_*` results.

## Human-in-the-loop and approvals

Meaningful external effects require an explicit approval state unless policy has already granted the exact scope. An approval surface names:

- the action and tool;
- the target object/account;
- requested permissions or external effects;
- important parameters that can change the outcome;
- whether approval applies once or creates an ongoing grant.

Use action labels with consequences, for example `Connect GitHub` or `Allow once`, not `OK`. Reject/cancel remains available and does not silently execute a fallback. While awaiting approval, the Run is paused and reconnectable; reloading must not duplicate the external effect.

## Streaming, cancellation, and reconnect

- Stream assistant text into a stable message container; avoid layout shifts and forced scrolling when the user has scrolled upward.
- The composer stays usable according to Runtime policy. Prevent duplicate submits for the same pending action.
- `Stop` cancels the Run, not merely the browser stream. Reconcile the final state from the server.
- On reconnect, restore persisted messages/events and identify whether the Run is active, completed, failed, or cancelled.
- Repeated AG-UI requests must eventually be idempotent before tools with side effects are enabled.

## Empty states

An empty state explains:

1. what the object is;
2. why the user might use it;
3. the next concrete action.

Examples include `Create conversation`, `Add memory`, `Install skill`, or `Connect MCP server`. Avoid giant decorative illustrations and generic “Nothing here” copy.

## Loading and errors

Use local skeletons or status rows when only a region is loading. Preserve navigation and already-loaded content. Full-view loading is appropriate only when the shell cannot be meaningfully rendered.

Errors must retain the attempted object/action and offer the safest next step. Distinguish validation, permission, connectivity, provider, tool, and server failures when the distinction changes recovery. Never include API keys or unredacted credentials.

## Feedback and confirmations

Confirm changes by naming the object and resulting state. Use reversible inline feedback for low-risk changes. Reserve modal confirmation for destructive, expensive, credential, permission, or external-effect actions. A toast alone is not sufficient for a failed action whose context disappears.

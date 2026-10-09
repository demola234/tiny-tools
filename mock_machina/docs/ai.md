# Using MockMachina with an AI assistant

`mockmachina mcp` lets the assistant you already use (Claude Code, Claude Desktop, Cursor, VS Code Copilot, ChatGPT) work with your contract. It can read routes, suggest the states your app is missing, write them, and compare the contract with git or a running API.

MockMachina doesn't call any AI itself and never needs an API key. Your assistant does the thinking. MockMachina does the reading and writing, with the same checks as `lint`, over the [Model Context Protocol](https://modelcontextprotocol.io).

## Set it up

From your app's folder, print the setup for your assistant:

```text
$ mockmachina mcp --print-config claude-code
claude mcp add mockmachina -- /Users/ada/go/bin/mockmachina mcp --dir /Users/ada/shop/.mockmachina
```

| Assistant | `--print-config` | Where the output goes |
| --- | --- | --- |
| Claude Code | `claude-code` | Run the printed command |
| Claude Desktop | `claude-desktop` | Merge into `claude_desktop_config.json` (Settings → Developer → Edit Config) |
| Cursor | `cursor` | Merge into `.cursor/mcp.json` in your project, or `~/.cursor/mcp.json` |
| VS Code | `vscode` | Merge into `.vscode/mcp.json` |

The setup uses full paths to the binary and the project, because some assistants start servers from another folder and without your shell's `PATH`. Run `--print-config` again if you move either.

## What to ask

Your assistant also lists these ready-made tasks, usually as slash commands or in a prompt menu:

| Prompt | What it does |
| --- | --- |
| `suggest_states` | Finds the states apps still need (401, 404, 422, 500, empty lists, slow responses) and adds the ones you pick |
| `draft_from_sample` | Turns a real API response into a route with an error state |
| `explain_diff` | Writes a pull-request comment on what changed in the contract and who needs to act |
| `check_live` | Compares the contract with a running API and says which side needs fixing |

Plain questions work too: "which routes have no error states?", "add a slow state to users.list", "what breaks if this branch merges?".

## What it can do

| Tool | Reads or writes | Does |
| --- | --- | --- |
| `list_routes` | reads | Routes, their states and which one is served by default |
| `get_route` | reads | One route in full, with every state's status, headers and body |
| `lint` | reads | The same errors and warnings as `mockmachina lint` |
| `diff` | reads | Contract changes between git refs, marked breaking, warning, info or safe ([details](diff.md)) |
| `diff_live` | reads | Differences between the contract and a running API ([details](diff.md#against-a-running-api)) |
| `add_route` | writes | A new route with its states |
| `add_state` | writes | A new state on an existing route |
| `set_state` | writes | Which state a route serves by default |

It can't delete or rename anything. Do that in the files.

## What keeps it safe

- **Nothing broken is ever written.** Every write is checked the way `lint` checks files, before it's saved. If it would break a file, nothing changes, and the assistant gets the same message `lint` would give, with a "did you mean" where there is one.
- **Everything the assistant writes is marked.** New routes and states get `generated: true`, and new routes also get `status: draft`. `lint` warns about each one until a person reviews it and deletes the `generated` line, and pull requests show it in the diff.
- **Your assistant asks before writing.** Assistants show each write and ask you first, unless you've told them not to.
- **Read-only mode.** `mockmachina mcp --read-only` offers only the tools that read. Add it to the setup with `--print-config … --read-only`.
- **Live checks only read.** `diff_live` sends only `GET`, `HEAD` and `OPTIONS`, and the assistant can't change that.
- **Tokens stay out of the chat.** If the live API needs auth, put the header in the setup, not in the conversation:

  ```sh
  mockmachina mcp --print-config claude-code --live-header "Authorization: Bearer $STAGING_TOKEN"
  ```

## Assistants that connect by URL

ChatGPT and other hosted assistants can't start a program on your machine, so they connect over HTTP instead:

```text
$ mockmachina mcp --http 127.0.0.1:4002
serving MCP on http://127.0.0.1:4002/mcp (Ctrl+C to stop)
```

To reach it from the internet, put a tunnel such as `cloudflared` or `ngrok` in front of it, and **always set a token**:

```sh
export MOCKMACHINA_MCP_TOKEN=$(openssl rand -hex 24)
mockmachina mcp --http 127.0.0.1:4002 --read-only
```

With a token, every request needs either an `Authorization: Bearer <token>` header or the token in the path: `https://your-tunnel.example/mcp/<token>`. Use the path form for assistants that can't set headers, and treat the URL as a password. Listening on anything other than this machine (`--http 0.0.0.0:4002`) needs a token. Pages open in your browser can't call the server, with or without one.

`--read-only` is a good default for a server on the internet.

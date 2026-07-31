# PatchMon CLI

The CLI reuses PatchMon's existing authenticated host and web SSH APIs.

## Build

```bash
cd cli-source-code
make build
```

## Use

```bash
./patchmon login --server https://patchmon.example.com
./patchmon hosts list
./patchmon hosts list --output json
./patchmon ssh admin@my-server
./patchmon ssh-tunnel my-server
```

## Completion

```bash
source <(patchmon completion bash)
patchmon completion zsh > ~/.zsh/completions/_patchmon
patchmon completion fish > ~/.config/fish/completions/patchmon.fish
```

Completion includes commands, options, and host names for `patchmon ssh user@host` and `patchmon ssh-tunnel host`. Host completion uses the current PatchMon login and silently falls back to command-only completion when the server is unavailable.

## SSH connection modes

- `patchmon ssh user@host`: **PTY Agent**, the same recorded agent-backed mode shown in PatchMon.
- `patchmon ssh-tunnel host`: **Agent Tunnel**, a raw tunnel that is not recorded.

For an HTTP development instance, pass `--insecure` to `login`. PTY Agent sessions require `ssh-bastion-enabled: true` in the target agent's `config.yml`. The CLI stores the PatchMon access token in the user's configuration directory with mode `0600`; Linux account passwords are entered only into the remote PTY prompt and are not stored by the CLI.

# TAC - Trigger Agent Connection

---

## About

This project consists of Agent Server and Trigger Server.
The Agent Server stores system prompts, configures LLM models, and defines execution roles.
The Trigger Server binds a workspace (local directory), connects to the Agent Server, automatically downloads tools, and invokes the LLM through loaded roles.
A Trigger Server can connect to multiple Agent Servers and load their roles. While loading roles, any associated tools are automatically downloaded.
This project is built by VibeCoding.

## Modules Included
### Agent Server
- **models**: Configuration hub for LLMs, including OpenAI chat completions and Anthropic APIs.
- **components**: Dynamic prompt templates, including static and embedded types. Embedded types inject workspace environment variables into prompt templates using the `{env_name}` syntax.
- **tools**: Executable scripts or binary files, including schemas, descriptions, and dependency configurations.
- **roles**: Bundles prompt components, model configurations, and tools to define a role, acting as a session template for invoking the LLM.

### Trigger Server
- **workspace**: The designated working folder, utilizing workspace files and environment context.
- **servers**: Gateway to connect to Agent Servers and select which roles to load.
- **events**: LLM invocation tasks that use workspace and environment variables to construct context for LLM completions.
- **triggers**: Binds local files or directories to events. Supports three execution modes: direct ("call once"), periodic ("call at specified intervals"), and edit ("call when files are modified"). 
- **envs**: Workspace environment variables, mapping local files as environment keys in the format `file:filename`.

### How to Use

#### Agent Server
1. Create components, tools, and model configurations.
2. Define and configure roles.
3. Start the server.

#### Trigger Server
1. Create or bind a workspace.
2. Connect to the Agent Server, select roles, and download their associated tools.
3. Start the daemon process.
4. Define execution events.
5. Create a trigger to bind files or folders to an event.
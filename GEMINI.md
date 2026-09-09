# Tool Usage Constraints
**CRITICAL**: Never use raw terminal commands (e.g., `grep`, `cat | Select-String`, `findstr`, etc.) for searching files or text.
Always use the built-in `grep_search` and `find_by_name` tools provided in your toolset. Raw terminal search commands are inefficient, prone to escaping errors, and strictly prohibited.

**EXCEPTION**: You are explicitly permitted to use the `ls` command in the terminal for listing files and directories without asking for permission.

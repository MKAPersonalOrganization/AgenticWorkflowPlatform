# Tool Extensions for Student Teaching Agent

This document analyzes tools from the `gemini-cli` repository that can be adopted to improve the performance and capabilities of our student teaching agent.

## Recommended Tools for Adoption

| Tool | Purpose | Impact on Student Teaching & Academic Learning |
| :--- | :--- | :--- |
| **`web-search`** | Search the web for academic resources, research papers, and textbooks. | **Information Depth**: Enables the agent to find up-to-date and specialized material beyond its training data. |
| **`web-fetch`** | Retrieve and convert web content (including books and articles) to Markdown. | **Reading Capabilities**: Essential for "reading books" that are available online or accessing specific academic articles. |
| **`read-many-files`** | Efficiently read multiple files or chapters in a single call. | **Performance**: Significantly reduces the number of tool execution cycles when analyzing large textbooks or multiple class notes. |
| **`memoryTool`** | Store and retrieve information across sessions (e.g., student progress, preferences). | **Personalization**: Allows the agent to act as a long-term mentor by remembering student context, past difficulties, and learning pace. |
| **`mcp-tool`** | Connect to Model Context Protocol (MCP) servers (e.g., Wolfram Alpha, OpenLibrary). | **Specialization**: Connects the agent to powerful external domain-specific engines for math, literature, or interactive simulations. |
| **`write-todos`** | Manage and track tasks, assignments, and study schedules. | **Organization**: Helps students stay on track with their academic goals and manage their homework efficiently. |
| **`grep` / `ripGrep`** | High-speed searching within local academic notes or repositories. | **Accessibility**: Quickly find specific concepts or references across a large volume of academic data. |
| **`get-internal-docs`** | Access project-specific curriculum, guidelines, or school policies. | **Alignment**: Ensures the agent adheres to the specific pedagogical standards and curriculum of "SchoolWithAI". |

## Performance Improvements

Adopting these tools will improve agent performance in the following ways:

1.  **Reduced Latency**: Tools like `read-many-files` and `grep` allow the agent to process large amounts of information in fewer steps, reducing wait times for students.
2.  **Enhanced Context Awareness**: The `memoryTool` maintains a "long-term memory" of student interactions, preventing the need for repetitive explanations and enabling more efficient, targeted tutoring.
3.  **Scalable Knowledge**: `web-search` and `mcp-tool` prevent the agent from being limited by its static training, allowing it to provide accurate information on new or highly specialized academic subjects.
4.  **Operational Efficiency**: `write-todos` offloads the burden of manual task tracking from the student, providing a structured learning path managed by the AI.

## Next Steps

- Integrate the `mcp-core` and these tool definitions into the `SchoolWithAI` agent framework.
- Configure a local `memoryTool` to start tracking student progress.
- Set up MCP servers for specialized subjects like Math (Wolfram) or Science.

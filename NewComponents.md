# New Components Roadmap (Inspired by LibreChat)

This document outlines a roadmap for "premium" components and features that can be added to the `templui` webapp to match or exceed LibreChat's user experience.

## Suggested Components

### 1. Mermaid Diagram Renderer
- **What**: Render code blocks with `mermaid` language as interactive diagrams.
- **Why**: Essential for visualizing technical concepts and workflows directly in chat.
- **LibreChat Equivalent**: `Mermaid.tsx`.

### 2. Audio Interaction (Voice-to-Text & Text-to-Voice)
- **What**: Microphone button in chat input for recording and an audio player for model responses.
- **Why**: Enhances accessibility and provides a more natural conversational interface.
- **LibreChat Equivalent**: `AudioRecorder.tsx`, `StreamAudio.tsx`.

### 3. Rich Mention System (@ and /)
- **What**: Expand the slash menu to support `@` for agents/models and `#` or similar for specific files/documents.
- **Why**: Allows precise control over context and model selection without navigating menus.
- **LibreChat Equivalent**: `Mention.tsx`.

### 4. Multi-Response Switcher
- **What**: UI controls to toggle between different model outputs for the same prompt.
- **Why**: Critical for users who want to "model hop" or compare results.
- **LibreChat Equivalent**: `ResultSwitcher.tsx`.

### 5. Interactive Conversation Starters
- **What**: Categorized prompt cards (e.g., "Analyze Data", "Write Code", "Plan Trip") on the empty state.
- **Why**: Improves onboarding and helps users understand the agent's capabilities.
- **LibreChat Equivalent**: `ConversationStarters.tsx`.

### 6. MCP (Model Context Protocol) Integration
- **What**: A settings panel to manage dynamic external data sources and tools.
- **Why**: Facilitates power-user features like connecting private databases or custom APIs.
- **LibreChat Equivalent**: `MCPConfigDialog.tsx`.

### 7. Ghost/Temporary Mode
- **What**: A "No History" toggle for private or one-off sessions.
- **Why**: Privacy-conscious users appreciate not having every test query saved.
- **LibreChat Equivalent**: `TemporaryChat.tsx`.

### 8. Enhanced Message Actions
- **What**: Floating or inline toolbars for Copy, Regenerate, Edit, and Rate.
- **Why**: Streamlines the interaction loop.
- **LibreChat Equivalent**: `MessageActions.tsx`.

### 9. Artifacts View (Side Panel)
- **What**: A retractable side panel to display code, documents, and rendered content (like Mermaid diagrams or HTML snippets) separate from the chat flow.
- **Why**: Keeps the main chat clean and allows for persistent viewing of complex outputs.
- **LibreChat Equivalent**: `Artifacts.tsx`, `ArtifactsPanel.tsx`.

### 10. Custom Prompt Library
- **What**: A UI to create, edit, and manage reusable prompt templates with support for dynamic variables (e.g., `{{input}}`).
- **Why**: Boosts productivity by allowing users to save and reuse complex prompts.
- **LibreChat Equivalent**: `PromptEditor.tsx`, `PromptVariables.tsx`.

### 11. Agent Memory/Context Management
- **What**: A panel to view and modify the "memory" or persistent context of an agent.
- **Why**: Gives users transparency and control over what the agent "remembers" across sessions.
- **LibreChat Equivalent**: `Memories.tsx`.

### 12. Multi-Model Comparison Mode
- **What**: Split-view or tabbed interface to compare responses from multiple models for the same prompt simultaneously.
- **Why**: Essential for evaluating different models' performance and accuracy.
- **LibreChat Equivalent**: `AddMultiConvo.tsx`.

### 13. Model Preset Management
- **What**: Save and load specific model configurations (temperature, top_p, system instructions) as reusable presets.
- **Why**: Allows users to quickly toggle between "Creative", "Precise", or "Technical" profiles.
- **LibreChat Equivalent**: `PresetsMenu.tsx`.

### 14. File Management Dashboard & RAG UI
- **What**: A dedicated view to manage uploaded documents, view their parsing status, and control which files are indexed for RAG (Retrieval-Augmented Generation).
- **Why**: Essential for power users working with large sets of private data.
- **LibreChat Equivalent**: `FileDashboardView.tsx`, `VectorStoreView.tsx`.

### 15. Public Conversation Sharing
- **What**: Generate unique, shareable URLs for conversations (with options to hide user identity).
- **Why**: Facilitates collaboration and knowledge sharing.
- **LibreChat Equivalent**: `ShareView.tsx`.

### 16. Global Search Bar
- **What**: A fast, indexed search to find specific messages, threads, or prompts across the entire history.
- **Why**: As history grows, finding past information becomes difficult without search.
- **LibreChat Equivalent**: `SearchBar.tsx`.
- **Status**: Implemented in [`components/searchbar/searchbar.templ`](file:///home/mabdelrahman/SchoolWithAI/components/searchbar/searchbar.templ).

### 17. Agent/Prompt Marketplace
- **What**: A UI to browse, "install", and share community-created agents and prompt templates.
- **Why**: Encourages community growth and makes it easier to find high-quality configurations.
- **LibreChat Equivalent**: `AgentMarketplaceButton.tsx`.

### 18. System-Wide Multi-Modal Support
- **What**: Specialized UI components for handling image, video, and audio uploads directly in the chat input with preview thumbnails.
- **Why**: Matches modern AI capabilities (GPT-4o, Claude 3.5 Sonnet).
- **LibreChat Equivalent**: `FilePreview.tsx`, `AttachFile.tsx`.

### 19. Global System Banners (Implemented)
- **What**: A top-level banner component for maintenance notices, new features, or status updates.
- **Why**: Keeps users informed about the platform's state.
- **LibreChat Equivalent**: `Banner.tsx`
- **Implementation**: [components/banner/banner.templ](file:///home/mabdelrahman/SchoolWithAI/components/banner/banner.templ).

### 20. Metadata-Driven Parameter Panel
- **What**: A dynamic settings panel that generates UI controls (sliders, switches, dropdowns) based on a JSON/YAML schema of the model's parameters.
- **Why**: Allows for rapid support of new model features without manually rebuilding the UI.
- **LibreChat Equivalent**: `DynamicSlider.tsx`, `DynamicDropdown.tsx`, `Panel.tsx`.

### 21. Granular Access Control (RBAC) UI
- **What**: A UI to manage user roles, permissions, and access to specific agents or models.
- **Why**: Critical for organizational use where different teams need different access levels.
- **LibreChat Equivalent**: `AccessRolesPicker.tsx`, `GenericGrantAccessDialog.tsx`.

### 22. Interactive Tool Logs
- **What**: A specialized message component that shows a "collapsible" log of tool executions (e.g., Python code running, web search results).
- **Why**: Debugging and transparency—users can see *how* the agent arrived at an answer.
- **LibreChat Equivalent**: `ToolCalls.tsx` (found in `Messages/Content`).

### 23. Real-time Audio Streaming & TTS
- **What**: Low-latency audio playback of model responses as they are being generated (streaming TTS).
- **Why**: Provides a more human-like, interactive experience, especially on mobile.
- **LibreChat Equivalent**: `StreamAudio.tsx`.

### 24. Rich Citations & Source Hovercards
- **What**: Interactive markers in AI responses that show the source document, page number, and snippet when hovered or clicked.
- **Why**: Essential for trust and verification in RAG (Retrieval-Augmented Generation) applications.
- **LibreChat Equivalent**: `Citation.tsx`, `SourceHovercard.tsx`, `Sources.tsx`.

### 25. Two-Factor Authentication (2FA) UI
- **What**: Secure login screens for TOTP (Google Authenticator) or email-based second factors.
- **Why**: Enhances account security for production-grade applications.
- **LibreChat Equivalent**: `TwoFactorScreen.tsx`.

### 26. Intelligent Screen Wake-Lock
- **What**: A background manager that prevents the user's screen from turning off while a long response is being generated or played back.
- **Why**: Prevents interruption of long-running tasks or audio streams.
- **LibreChat Equivalent**: `WakeLockManager.tsx`.

### 27. Advanced Assistant/Agent Builder UI
- **What**: A multi-tab configuration interface for defining agent "Capabilities" (code interpreter, vision, retrieval) and uploading "Knowledge" files.
- **Why**: Simplifies the creation of specialized agents without advanced technical knowledge.
- **LibreChat Equivalent**: `CapabilitiesForm.tsx`, `Knowledge.tsx`, `AssistantPanel.tsx`.

### 28. MCP Resource Carousel
- **What**: A horizontally scrollable carousel for displaying multiple images, documents, or data visualizations returned by MCP tools.
- **Why**: Efficiently uses space when multiple large outputs are generated.
- **LibreChat Equivalent**: `MCPUIResourceCarousel.tsx`.

### 29. Multi-Select Model/Configuration Dropdown
- **What**: A sophisticated dropdown that allows users to select multiple models for comparison or multiple "Tools" for a session.
- **Why**: Necessary for advanced workflows like "Multi-Model Comparison".
- **LibreChat Equivalent**: `MultiSelectDropDown.tsx`, `MultiSelectPop.tsx`.

### 30. Bulk Conversation Management
- **What**: A UI pattern (checkmarks + toolbar) for selecting multiple threads in the history and deleting or moving them at once.
- **Why**: Critical for maintaining large histories over time.
- **LibreChat Equivalent**: `Conversations.tsx`, `ConvoOptions.tsx`.

### 31. Unified Tool/Plugin Selector
- **What**: A searchable dialog or grid view to browse, enable, and configure external tools and plugins.
- **Why**: Centralizes tool management as the ecosystem grows.
- **LibreChat Equivalent**: `ToolSelectDialog.tsx`, `AssistantToolsDialog.tsx`.

### 32. Interactive "Thought" or "Reasoning" Blocks
- **What**: A specialized message block for displaying a model's "chain of thought" or reasoning process, often styled with a different background or border.
- **Why**: Helps users understand the model's logic and increases trust in complex tasks.
- **LibreChat Equivalent**: `Thinking.tsx` (pattern observed in modern AI UIs and LibreChat's message rendering).

### 33. Inline Code Execution (Sandbox)
- **What**: A "Play" button on code blocks that executes the code (e.g., Python, JavaScript) in a secure sandbox and displays the output below.
- **Why**: Transforms the chat into a powerful development and data analysis tool.
- **LibreChat Equivalent**: `RunCode.tsx`.

### 34. Comprehensive Conversation Export
- **What**: A modal that allows users to export the current or multiple conversations into various formats: PDF, Markdown, HTML, or even a long PNG screenshot.
- **Why**: Essential for students and professionals who need to document their AI interactions.
- **LibreChat Equivalent**: `ExportModal.tsx`.

### 35. Usage & Token Balance Dashboard
- **What**: A UI component (often in settings or account menu) that shows the user's current token usage, remaining credits, or subscription status.
- **Why**: Transparency for paid or quota-limited services.
- **LibreChat Equivalent**: `Balance` settings.

### 36. Advanced Speech Configuration Panel
- **What**: Detailed settings for TTS (Text-to-Speech) and STT (Speech-to-Text), including voice selection, playback speed, and auto-detect language options.
- **Why**: Critical for accessibility and a "premium" hands-free experience.
- **LibreChat Equivalent**: `Speech` settings.

### 37. Web Content Context Fetcher
- **What**: An "Add URL" button that fetches the text content from a provided link and automatically includes it in the model's context.
- **Why**: Saves users from manually copying and pasting long articles or documentation.
- **LibreChat Equivalent**: `plugin.ts` (Web).

### 38. LaTeX/Mathematical Equation Rendering
- **What**: Support for rendering LaTeX formulas dynamically within the chat bubbles using libraries like KaTeX or MathJax.
- **Why**: Non-negotiable for educational and scientific use cases.
- **LibreChat Equivalent**: Specialized markdown rendering logic.

### 39. Bulk History Export/Import
- **What**: Tools to download the entire user history as a JSON/ZIP file or import sessions from other platforms.
- **Why**: Prevents vendor lock-in and provides peace of mind for heavy users.
- **LibreChat Equivalent**: `Data` settings.

### 40. Customizable UI Density & Font Settings
- **What**: "Comfy" vs "Compact" layout modes and font size/style customization.
- **Why**: Personalization is key to long-term user satisfaction.
- **LibreChat Equivalent**: `Chat` settings, `Personalization`.

---

## Strategy for Implementation

- **Theme Consistency**: Ensure all new components strictly follow the `templui` design system and current dark/light mode tokens.
- **Alpine.js Focus**: Leverage Alpine.js for interactivity to keep the frontend lightweight and compatible with Go/Templ.
- **Progressive Enhancement**: Add these features as modular components that can be toggled on/off.

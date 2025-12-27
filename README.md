# School with AI

A chat application built with Go, Templ, Tailwind CSS, and Alpine.js, implementing the AG-UI protocol for AI agent interactions.

## Tech Stack

- **Backend**: Go with Templ templating
- **Frontend**: Alpine.js for reactivity
- **Styling**: Tailwind CSS v4
- **Components**: templui (shadcn/ui equivalent for Go/Templ)
- **Protocol**: AG-UI for server-sent events

## Prerequisites

- Go 1.21+
- Task (task runner)
- templui CLI

## Setup

1. Install dependencies:
```bash
task setup-ubuntu
```

2. Initialize templui:
```bash
templui init
```

3. Create a `.env` file:
```bash
echo "SERVER_ADDR=localhost:8090" > .env
```

4. Start the development server:
```bash
task dev
```

The application will be available at http://localhost:7331 (proxy) or http://localhost:8090 (direct).

## Adding New Components with templui

templui is a component library for Go/Templ, similar to shadcn/ui for React. It provides pre-built, customizable components that follow the same design system.

### Adding a New Component

To add a new component to your project:

```bash
templui add <component-name>
```

**Examples:**

```bash
# Add a single component
templui add button

# Add multiple components at once
templui add card input button separator

# Add all available components
templui add --all
```

### Available Components

Common templui components include:
- `button` - Button component with variants
- `card` - Card layout with Header, Title, Description, Content, Footer
- `input` - Form input component
- `separator` - Visual divider
- `badge` - Small status indicators
- `alert` - Alert/notification boxes
- `dialog` - Modal dialogs
- `dropdown-menu` - Dropdown menus
- `tabs` - Tabbed interfaces
- `tooltip` - Tooltips

See the full list at: https://templui.com/docs/components

### Using Components in Templates

After adding a component, import and use it in your `.templ` files:

```go
package components

import (
    "school-with-ai/components/button"
    "school-with-ai/components/card"
)

templ MyComponent() {
    @card.Card(card.Props{
        Class: "w-full max-w-md",
    }) {
        @card.Header(card.HeaderProps{}) {
            @card.Title(card.TitleProps{}) {
                My Card Title
            }
        }
        @card.Content(card.ContentProps{}) {
            <p>Card content goes here</p>
        }
        @card.Footer(card.FooterProps{}) {
            @button.Button(button.Props{
                Type: button.TypeSubmit,
            }) {
                Submit
            }
        }
    }
}
```

### Component Structure

Components are added to the `components/<component-name>/` directory with:
- `<component-name>.templ` - Component template
- `<component-name>.go` - Component props and types
- Additional helper files as needed

### Customizing Components

templui components are copied into your project (not installed as dependencies), so you can:
1. Modify the component files directly
2. Adjust styling by editing Tailwind classes
3. Change component behavior in the `.templ` files
4. Update props structure in the `.go` files

### Theme Configuration

templui uses CSS variables for theming. Customize colors in `assets/css/input.css`:

```css
@theme {
    --color-primary: oklch(0.55 0.25 262);
    --color-secondary: oklch(0.75 0.1 262);
    /* ... more theme variables */
}
```

## Development Workflow

The `task dev` command runs:
1. **Tailwind CSS** - Watches and compiles CSS
2. **Templ** - Generates Go code from templates with hot reload
3. **Go server** - Runs the application with auto-restart

Any changes to `.templ`, `.css`, or `.go` files will automatically rebuild and reload.

## Project Structure

```
.
├── components/          # Templ components
│   ├── button/         # Button component (from templui)
│   ├── card/           # Card component (from templui)
│   ├── input/          # Input component (from templui)
│   └── hello.templ     # Main chat UI
├── handlers/           # HTTP handlers
│   └── chat.go         # Chat SSE endpoint
├── internal/
│   └── agui/          # AG-UI protocol implementation
├── services/
│   └── openai/        # OpenAI integration
├── assets/
│   ├── css/           # Tailwind CSS files
│   └── js/            # Alpine.js components
├── main.go            # Application entry point
└── Taskfile.yaml      # Task runner configuration
```

## AG-UI Protocol

The application implements the AG-UI protocol for streaming AI responses:

- `TEXT_MESSAGE_START` - Message streaming begins
- `TEXT_MESSAGE_CONTENT` - Streamed content delta
- `TEXT_MESSAGE_END` - Message streaming completes

See `plan.md` for the full implementation roadmap.

# School with AI

A chat application built with Go, Templ, Tailwind CSS, and Alpine.js, implementing the AG-UI protocol for AI agent interactions.

## Tech Stack

- **Backend**: Go with Templ templating
- **Frontend**: Alpine.js for reactivity
- **Styling**: Tailwind CSS v4
- **Components**: templui (shadcn/ui equivalent for Go/Templ)
- **Protocol**: AG-UI for server-sent events
- **Linear Algebra**: gosl library (LAPACK/OpenBLAS integration)

## Architecture

### Multi-Agent System

The application uses an event-driven multi-agent architecture with specialized instructors:

- **student-facing-agent**: Routes user questions to specialists
- **math-instructor**: General mathematics, delegates to linear algebra professor
- **physics-instructor**: Physics problems
- **chemistry-instructor**: Chemistry topics
- **statistics-instructor**: Statistical analysis
- **linearalgebra-professor**: Advanced linear algebra (vectors, matrices, eigenvalues)

### Access Control: Capability-Based Security

The system enforces **which agents can call other agents** through capability-based access control:

**How it works:**
1. Agents can only use **tools** registered in their tool registry
2. `emit_linearalgebra_question` tool is **only** registered in math-instructor's registry
3. Linear algebra professor **only** responds to `LinearAlgebraQuestion` events
4. Worker pool routes events to appropriate agents
5. No direct agent-to-agent invocation possible

**Example - Math Instructor calling Linear Algebra Professor:**
```go
// In math-instructor's tool registry:
registry.Register(sharedtools.NewEmitLinearAlgebraQuestionTool())  // ✅ Has access

// In physics-instructor's tool registry:
// emit_linearalgebra_question tool NOT registered  // ❌ No access
```

This creates a **hard architectural constraint**: only math-instructor can delegate to the linear algebra specialist. Other agents physically cannot invoke it because they lack the necessary tool.

**Benefits:**
- Prevents unauthorized agent interactions
- Clear separation of concerns
- Type-safe access control enforced at compile time
- Optimized library usage only where needed (gosl/LAPACK only loaded for linear algebra)

## Authentication

The application uses [Ory Kratos](https://www.ory.sh/kratos/) for identity and user management.

### Features (Current State)

| Feature | Status |
|---------|--------|
| User Registration | ✅ Implemented |
| Email Verification | ✅ Implemented |
| User Login | ✅ Implemented |
| Password Recovery | 🔄 In Progress |
| User Settings | ⬜ Not Started |
| Logout | ⬜ Not Started |
| Protected Routes | ⬜ Not Started |

### Auth Stack

- **Ory Kratos**: Identity server handling registration, login, verification, recovery
- **PostgreSQL**: Identity storage
- **MailSlurper**: Email testing (development only)

### Auth Routes

| Route | Description |
|-------|-------------|
| `GET /auth/login` | Login page |
| `GET /auth/registration` | Registration page |
| `GET /auth/verification` | Email verification page |
| `GET /auth/recovery` | Password recovery (not implemented) |
| `GET /auth/settings` | User settings (not implemented) |
| `GET /auth/logout` | Logout handler (not implemented) |

### Running with Authentication

1. Start the infrastructure:
```bash
docker compose up -d
```

2. Verify services are running:
```bash
# Kratos health check
curl http://127.0.0.1:4433/health/ready

# MailSlurper UI (for verification emails)
open http://127.0.0.1:4436
```

3. Start the application:
```bash
task dev
```

4. Access the auth pages:
- Registration: http://127.0.0.1:8090/auth/registration
- Login: http://127.0.0.1:8090/auth/login

### Local Signup Flow

1. **Register**: Go to http://127.0.0.1:8090/auth/registration
   - Fill in email, password, first name, last name
   - Click "Sign up"

2. **Get Verification Code**: Open MailSlurper at http://127.0.0.1:4436
   - Find the verification email sent to your address
   - Copy the 6-digit verification code

3. **Verify Email**: You'll be redirected to the verification page
   - Enter the verification code from the email
   - Click "Submit"

4. **Login**: After verification, go to http://127.0.0.1:8090/auth/login
   - Enter your email and password
   - Click "Sign in"

### Environment Variables (Auth)

```bash
# Required
APP_URL=http://127.0.0.1:8090
KRATOS_PUBLIC_URL=http://127.0.0.1:4433
KRATOS_ADMIN_URL=http://127.0.0.1:4434
```

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
│   ├── auth/           # Authentication pages (login, registration, verification)
│   ├── button/         # Button component (from templui)
│   ├── card/           # Card component (from templui)
│   ├── input/          # Input component (from templui)
│   └── hello.templ     # Main chat UI
├── domain/             # Domain models
│   └── user.go         # User model matching Kratos identity
├── handlers/           # HTTP handlers
│   └── chat.go         # Chat SSE endpoint
├── infra/
│   ├── auth/           # Kratos client, middleware, handlers
│   └── agui/           # AG-UI protocol implementation
├── kratos/             # Ory Kratos configuration
│   ├── kratos.yml      # Kratos server config
│   └── identity.schema.json  # User identity schema
├── services/
│   └── openai/         # OpenAI integration
├── assets/
│   ├── css/            # Tailwind CSS files
│   └── js/             # Alpine.js components
├── main.go             # Application entry point
├── docker-compose.yml  # Infrastructure (Kratos, PostgreSQL, MailSlurper)
└── Taskfile.yaml       # Task runner configuration
```

## AG-UI Protocol

The application implements the AG-UI protocol for streaming AI responses:

- `TEXT_MESSAGE_START` - Message streaming begins
- `TEXT_MESSAGE_CONTENT` - Streamed content delta
- `TEXT_MESSAGE_END` - Message streaming completes

See `plan.md` for the full implementation roadmap.

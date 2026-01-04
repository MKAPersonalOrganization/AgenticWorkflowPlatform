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

### Declarative Event-Driven Multi-Agent System

The application uses a **declarative agent contract architecture** where agents define their behavior through contracts rather than manual wiring.

#### Agent Contracts

Each agent declares its event-driven contract in a `contract.go` file:

```go
var Contract = &eventbus.AgentContract{
    Name:      "math-instructor",
    ListensTo: []agents.EventType{agents.EventTypeMathQuestion},
    Emits:     []agents.EventType{
        agents.EventTypeQuestionAnswered,
        agents.EventTypePhysicsQuestion,
        agents.EventTypeLinearAlgebraQuestion,
    },
    Tools:     domainTools(),  // Specialized tools
    Factory:   CreateMathInstructor,
}
```

**What this provides:**
- **Automatic Event Routing**: Events are routed to agents based on `ListensTo` declarations
- **Auto-Generated Tools**: Emission tools (e.g., `emit_physics_question`) are generated from `Emits`
- **Collaboration Tools**: Whiteboard and communication tools are automatically provided
- **Zero Manual Wiring**: No need to update routing tables or worker pools when adding agents

#### Specialized Agents

- **student-facing-agent**: Routes user questions to appropriate specialists
- **math-instructor**: General mathematics, can delegate to linear algebra and other specialists
- **physics-instructor**: Physics problems and applications
- **chemistry-instructor**: Chemistry topics and reactions
- **statistics-instructor**: Statistical analysis and probability
- **linearalgebra-instructor**: Advanced linear algebra (vectors, matrices, eigenvalues)
- **diagramming-instructor**: Creates diagrams using Mermaid

#### How It Works

1. **Agent Registration**: Contracts are registered in `main.go`:
   ```go
   eventbus.GlobalContracts.Register(math.Contract)
   eventbus.GlobalContracts.Register(physics.Contract)
   ```

2. **Automatic Tool Generation**: The system generates emission tools for each agent:
   ```go
   // For math-instructor that can emit physics questions:
   emit_physics_question(question="...")  // Auto-generated
   ```

3. **Event-Driven Collaboration**: Agents collaborate by emitting events:
   ```
   User Question → student-facing-agent
                → emits math_question event
                → math-instructor receives event
                → processes and writes to whiteboard
                → emits question_answered event
                → student-facing-agent receives callback
   ```

4. **Shared Whiteboard**: All agents collaborate through a shared whiteboard:
   - `read_whiteboard`: Read current content
   - `write_whiteboard`: Update with new content
   - `emit_question_answered`: Signal completion

#### Adding a New Agent

See [docs/ADDING_NEW_AGENT.md](docs/ADDING_NEW_AGENT.md) for the complete guide.

**Quick overview:**
1. Create `services/agents/<name>/contract.go` with the agent contract
2. Implement the agent factory function
3. Add event type constant to `instructor_events.go`
4. Register contract in `main.go`

That's it! The system handles routing, tool generation, and event wiring automatically.

#### Architecture Benefits

- **Declarative**: Agents declare what they do, not how routing works
- **Decoupled**: Agents don't know about each other, only event types
- **Scalable**: Adding agents doesn't require updating multiple files
- **Type-Safe**: Event contracts enforced at compile time
- **Self-Documenting**: Contracts serve as documentation of agent capabilities

## Authentication

The application uses [Ory Kratos](https://www.ory.sh/kratos/) for identity and user management with a complete self-service flow implementation.

### Auth Stack

- **Ory Kratos**: Identity server handling all self-service flows
- **PostgreSQL**: Identity storage
- **MailSlurper**: Email testing (development only)
- **Go Middleware**: Session validation and route protection

### Auth Routes

| Route | Description | Protection |
|-------|-------------|------------|
| `GET /auth/login` | Login page | Public |
| `GET /auth/registration` | Registration page | Public |
| `GET /auth/verification` | Email verification page | Public |
| `GET /auth/recovery` | Password recovery page | Public |
| `GET /auth/settings` | User settings (profile, password, 2FA) | Protected |
| `GET /auth/logout` | Logout handler | Public |
| `GET /auth/error` | Error page for auth flows | Public |

### User Interface

The application displays user information in the left navigation bar:

**Authenticated Users:**
- User email and ID displayed in settings popover
- Quick access to Account Settings (`/auth/settings`)
- Theme toggle (Light/Dark/System)
- Logout button

**Unauthenticated Users:**
- Login button
- Register button
- Theme toggle

The settings icon is located at the bottom left of the navbar and shows context-appropriate options based on authentication state.

### Protected Routes

All chat and API routes now require authentication:

| Route | Description | Protection |
|-------|-------------|------------|
| `GET /` | Main chat interface | Protected |
| `POST /send` | Send message endpoint | Protected |
| `GET /api/jobs/active` | Get active jobs | Protected |
| `GET /api/jobs/{jobId}/stream` | Stream job events (SSE) | Protected |
| `DELETE /api/jobs/{jobId}` | Cancel a job | Protected |
| `GET /api/threads` | Get all threads | Protected |
| `GET /api/threads/{threadId}/messages` | Get thread messages | Protected |
| `DELETE /api/threads/{threadId}` | Delete a thread | Protected |

**Behavior:**
- Unauthenticated requests redirect to `/auth/login?return_to={original_url}`
- After successful login, users are redirected back to the original URL
- User context available in all handlers via `auth.GetUserFromContext()`

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

4. Access the application:
- Home/Chat: http://127.0.0.1:8090/
- Registration: http://127.0.0.1:8090/auth/registration
- Login: http://127.0.0.1:8090/auth/login
- Settings: http://127.0.0.1:8090/auth/settings (requires authentication)

### Complete Auth Flow

#### 1. Registration
1. Go to http://127.0.0.1:8090/auth/registration
2. Fill in email, password, first name, last name
3. Click "Sign up"
4. Redirected to verification page

#### 2. Email Verification
1. Open MailSlurper at http://127.0.0.1:4436
2. Find the verification email sent to your address
3. Copy the 6-digit verification code
4. Enter the code on the verification page
5. Click "Submit"
6. Redirected to settings page

#### 3. Login
1. Go to http://127.0.0.1:8090/auth/login
2. Enter your email and password
3. Click "Sign in"
4. Access protected routes

#### 4. Password Recovery
1. Click "Forgot your password?" on login page
2. Enter your email address
3. Check MailSlurper for recovery code
4. Enter the code and new password
5. Redirected to settings page

#### 5. Settings Management
1. Login to your account
2. Navigate to http://127.0.0.1:8090/auth/settings
3. Update profile information (first name, last name)
4. Change password
5. Enable/disable two-factor authentication (TOTP)
6. View recovery codes

#### 6. Logout
1. Click "Sign out" in settings page
2. Session cleared and redirected to login

### Environment Variables (Auth)

```bash
# Required
APP_URL=http://127.0.0.1:8090
KRATOS_PUBLIC_URL=http://127.0.0.1:4433
KRATOS_ADMIN_URL=http://127.0.0.1:4434

# Optional (for production)
KRATOS_BROWSER_URL=http://127.0.0.1:4433  # External URL for browser redirects
```

### Authentication Architecture

**Components:**
- `domain/user.go` - User model with Kratos identity traits
- `domain/job.go` - Job model with UserID for tracking user actions
- `infra/auth/kratos_client.go` - Kratos API client wrapper
- `infra/auth/middleware.go` - Session validation middleware (`RequireAuth`, `OptionalAuth`)
- `infra/auth/session.go` - Session context helpers
- `infra/auth/handlers.go` - Auth page handlers (login, registration, etc.)
- `infra/auth/flow_helpers.go` - Flow URL generation and helper functions
- `infra/chat_handlers.go` - Chat handlers with user context integration
- `components/auth/` - Auth page templates (login, registration, verification, recovery, settings, error)

**Session Management:**
- Sessions stored in secure HTTP-only cookies by Kratos
- Middleware validates sessions on each request via Kratos `toSession` API
- User and session data stored in request context
- Session expiry handled automatically with re-authentication flows

**Route Protection:**
- All chat routes (`/`, `/send`, `/api/*`) protected with `RequireAuth` middleware
- Settings page protected with `RequireAuth` middleware
- Unauthenticated requests redirected to login with `return_to` parameter
- User context accessible in handlers via `auth.GetUserFromContext()`

**User Context Integration:**
- Jobs track UserID for associating messages with users
- Thread creation uses user-specific IDs (`thread_{userID}_{timestamp}`)
- User actions logged with user ID and email for audit trail
- Future: Thread filtering by user ownership

**Security Features:**
- CSRF protection on all forms
- Email verification required after registration
- Password recovery with time-limited codes
- AAL2 (two-factor authentication) support
- User enumeration protection (recovery flow doesn't reveal if email exists)
- Secure session cookies with HttpOnly and SameSite flags

## Prerequisites

### Docker Setup

The application can be run entirely in Docker containers for a consistent development environment.

**Configure environment variables:**

Before starting the containers, set your OpenAI API key on the host machine:
```bash
export OPENAI_API_KEY=your_actual_api_key_here
```

**Build and start all services:**
```bash
docker compose up -d --build
```

Or build without starting:
```bash
docker compose build
```

This will start:
- **app**: Main application server (port 8090)
- **postgres**: PostgreSQL database for Kratos (port 5433)
- **kratos**: Ory Kratos identity server (ports 4433, 4434)
- **kratos-migrate**: Database migration service
- **mailslurper**: Email testing service (ports 4436, 4437)

**Check service status:**
```bash
docker compose ps
```

**View application logs:**
```bash
docker compose logs app -f
```

**Rebuild and restart the app container:**
```bash
docker compose up app -d --build
```

**Stop all services:**
```bash
docker compose down
```

**Environment variables passed to Docker:**
- `OPENAI_API_KEY`: Captured from host environment for OpenAI API access
- Other configuration in `docker-compose.yml`

### Local Development

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
├── services/
│   ├── agents/
│   │   ├── eventbus/              # Event routing and contract registry
│   │   │   ├── contract.go        # AgentContract type definition
│   │   │   ├── eventbus.go        # Event subscription management
│   │   │   ├── emission.go        # Auto-generates emission tools
│   │   │   └── registry.go        # GlobalContracts singleton
│   │   ├── math/                  # Math instructor agent
│   │   │   ├── contract.go        # Math agent contract
│   │   │   ├── math.go            # Agent factory and config
│   │   │   └── tools/             # Math-specific tools
│   │   ├── physics/               # Physics instructor agent
│   │   ├── chemistry/             # Chemistry instructor agent
│   │   ├── statistics/            # Statistics instructor agent
│   │   ├── studentfacing/         # Router agent
│   │   ├── diagramming/           # Diagramming agent
│   │   ├── shared/
│   │   │   └── tools/             # Shared collaboration tools
│   │   └── instructor_events.go   # Event type constants
│   ├── sdk/                       # Agent SDK and core runtime
│   │   ├── agent.go               # Agent execution engine
│   │   ├── tool_executor.go       # Tool execution handling
│   │   └── types.go               # Core types and interfaces
│   └── llm/                       # LLM client abstraction
├── infra/
│   ├── worker_pool.go             # Event routing via contracts
│   ├── ui_notifier.go             # Domain → UI event translation
│   ├── auth/                      # Authentication infrastructure
│   │   ├── kratos_client.go       # Ory Kratos API client
│   │   ├── middleware.go          # Session validation
│   │   └── handlers.go            # Auth page handlers
│   └── chat_handlers.go           # Chat API handlers
├── domain/
│   ├── domain.go                  # Core domain types
│   ├── user.go                    # User model
│   └── job.go                     # Job processing model
├── components/                    # Templ components
│   ├── auth/                      # Authentication pages
│   │   ├── layout.templ           # Base layout for auth pages
│   │   ├── ui_nodes.templ         # Kratos UI node renderer
│   │   ├── login.templ            # Login page
│   │   ├── registration.templ     # Registration page
│   │   ├── verification.templ     # Email verification page
│   │   ├── recovery.templ         # Password recovery page
│   │   ├── settings.templ         # User settings page
│   │   └── error.templ            # Error page
│   ├── button/                    # Button component (from templui)
│   ├── card/                      # Card component (from templui)
│   ├── input/                     # Input component (from templui)
│   ├── form/           # Form components (from templui)
│   └── hello.templ     # Main chat UI
├── domain/             # Domain models
│   ├── user.go         # User model matching Kratos identity
│   └── job.go          # Background job model with UserID
├── infra/
│   ├── auth/           # Authentication infrastructure
│   │   ├── kratos_client.go      # Kratos API client wrapper
│   │   ├── middleware.go         # Session validation middleware
│   │   ├── session.go            # Session context helpers
│   │   ├── handlers.go           # Auth page handlers
│   │   ├── flow_helpers.go       # Flow URL generation and helpers
│   │   └── *_test.go            # Unit tests (29 tests)
│   ├── chat_handlers.go           # Chat SSE endpoint with user context
│   ├── emitter.go                 # SSE event emitter
│   ├── job_queue.go               # Background job queue
│   └── worker_pool.go             # Worker pool for async processing
├── kratos/             # Ory Kratos configuration
│   ├── kratos.yml      # Kratos server config (flows, URLs, SMTP)
│   └── identity.schema.json  # User identity schema (email, name)
├── services/
│   ├── agents/         # Specialized AI agents
│   │   ├── studentfacing/        # Router agent
│   │   ├── math/                 # Math instructor
│   │   ├── physics/              # Physics instructor
│   │   ├── chemistry/            # Chemistry instructor
│   │   ├── statistics/           # Statistics instructor
│   │   └── advancedmath/         # Linear algebra professor
│   └── llm/            # OpenAI integration
├── assets/
│   ├── css/            # Tailwind CSS files
│   └── js/             # Alpine.js components
├── docs/
│   ├── ADDING_NEW_AGENT.md        # Guide for adding new agents
│   ├── EVENT_ARCHITECTURE.md      # Event system architecture
│   ├── EVENT_FLOWS.md             # Event flow diagrams
│   └── todos.md                   # Implementation status
├── main.go             # Application entry point with contract registration
├── docker-compose.yml  # Infrastructure (Kratos, PostgreSQL, MailSlurper)
└── Taskfile.yaml       # Task runner configuration
```

## Documentation

- **[ADDING_NEW_AGENT.md](docs/ADDING_NEW_AGENT.md)** - Complete guide for adding new specialized agents
- **[EVENT_ARCHITECTURE.md](docs/EVENT_ARCHITECTURE.md)** - Detailed event system architecture
- **[EVENT_FLOWS.md](docs/EVENT_FLOWS.md)** - Event flow diagrams and patterns
- **[AUTH_IMPLEMENTATION_TODO.md](AUTH_IMPLEMENTATION_TODO.md)** - Authentication implementation status (95% complete)

## AG-UI Protocol

The application implements the AG-UI protocol for streaming AI responses:

- `TEXT_MESSAGE_START` - Message streaming begins
- `TEXT_MESSAGE_CONTENT` - Streamed content delta
- `TEXT_MESSAGE_END` - Message streaming completes

See `plan.md` for the full implementation roadmap.

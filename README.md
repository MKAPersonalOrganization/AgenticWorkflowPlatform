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

The application uses [Ory Kratos](https://www.ory.sh/kratos/) for identity and user management with a complete self-service flow implementation.

### Features

| Feature | Status |
|---------|--------|
| User Registration | ✅ Implemented |
| Email Verification | ✅ Implemented |
| User Login | ✅ Implemented |
| Password Recovery | ✅ Implemented |
| User Settings | ✅ Implemented |
| Logout | ✅ Implemented |
| Session Management | ✅ Implemented |
| Protected Routes | ⬜ Planned (Step 19) |
| User UI Elements | ⬜ Planned (Step 20) |

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
- `infra/auth/kratos_client.go` - Kratos API client wrapper
- `infra/auth/middleware.go` - Session validation middleware (`RequireAuth`, `OptionalAuth`)
- `infra/auth/session.go` - Session context helpers
- `infra/auth/handlers.go` - Auth page handlers (login, registration, etc.)
- `infra/auth/flow_helpers.go` - Flow URL generation and helper functions
- `components/auth/` - Auth page templates (login, registration, verification, recovery, settings, error)

**Session Management:**
- Sessions stored in secure HTTP-only cookies by Kratos
- Middleware validates sessions on each request via Kratos `toSession` API
- User and session data stored in request context
- Session expiry handled automatically with re-authentication flows

**Security Features:**
- CSRF protection on all forms
- Email verification required after registration
- Password recovery with time-limited codes
- AAL2 (two-factor authentication) support
- User enumeration protection (recovery flow doesn't reveal if email exists)

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
│   ├── auth/           # Authentication pages
│   │   ├── layout.templ           # Base layout for auth pages
│   │   ├── ui_nodes.templ         # Kratos UI node renderer
│   │   ├── login.templ            # Login page
│   │   ├── registration.templ     # Registration page
│   │   ├── verification.templ     # Email verification page
│   │   ├── recovery.templ         # Password recovery page
│   │   ├── settings.templ         # User settings page
│   │   └── error.templ            # Error page
│   ├── button/         # Button component (from templui)
│   ├── card/           # Card component (from templui)
│   ├── input/          # Input component (from templui)
│   ├── form/           # Form components (from templui)
│   └── hello.templ     # Main chat UI
├── domain/             # Domain models
│   ├── user.go         # User model matching Kratos identity
│   └── job.go          # Background job model
├── infra/
│   ├── auth/           # Authentication infrastructure
│   │   ├── kratos_client.go      # Kratos API client wrapper
│   │   ├── middleware.go         # Session validation middleware
│   │   ├── session.go            # Session context helpers
│   │   ├── handlers.go           # Auth page handlers
│   │   ├── flow_helpers.go       # Flow URL generation and helpers
│   │   └── *_test.go            # Unit tests (29 tests)
│   ├── chat_handlers.go           # Chat SSE endpoint
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
├── main.go             # Application entry point with auth routes
├── docker-compose.yml  # Infrastructure (Kratos, PostgreSQL, MailSlurper)
├── Taskfile.yaml       # Task runner configuration
└── AUTH_IMPLEMENTATION_TODO.md  # Implementation plan (18/20 steps complete)
```

## AG-UI Protocol

The application implements the AG-UI protocol for streaming AI responses:

- `TEXT_MESSAGE_START` - Message streaming begins
- `TEXT_MESSAGE_CONTENT` - Streamed content delta
- `TEXT_MESSAGE_END` - Message streaming completes

See `plan.md` for the full implementation roadmap.

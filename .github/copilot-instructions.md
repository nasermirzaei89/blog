# Copilot Instructions for Go Blog System

## Project Overview
This is a minimal but powerful blog system written entirely in Go. It follows WordPress-like approaches for content management but without a separate admin panel. The system emphasizes **progressive enhancement** - it works perfectly without JavaScript but provides enhanced user experience when JavaScript is enabled.

## Core Principles

### 1. **Progressive Enhancement First**
- **Always ensure functionality works without JavaScript**
- JavaScript (Alpine.js/Alpine AJAX) should only enhance the experience
- Forms must submit properly with standard HTTP requests as fallback
- Never break core functionality for non-JS users

### 2. **Minimal Dependencies**
- Only add dependencies when absolutely necessary
- Prefer standard library solutions over external packages
- Current essential dependencies:
  - `github.com/gorilla/*` for HTTP utilities (sessions, CSRF, handlers)
  - `github.com/gosimple/slug` for URL-friendly slugs
  - `github.com/microcosm-cc/bluemonday` for HTML sanitization
  - Database drivers as needed

### 3. **WordPress-Inspired UX**
- Inline editing capabilities
- WYSIWYG editor (TipTap) for rich content
- Slug generation from titles
- Excerpt generation (160 chars, word boundaries)
- Time-based formatting and "edited" indicators

## Technical Stack

### Backend (Go)
- **Architecture**: Single binary with embedded templates
- **Database**: SQLite with migration support
- **Templates**: Go's `html/template` with modular structure
- **Security**: CSRF protection, HTML sanitization, bcrypt passwords
- **Patterns**: Repository pattern, context-based user sessions

### Frontend
- **CSS**: Tailwind CSS (utility-first, compiled to single file)
- **JavaScript**: Alpine.js + Alpine AJAX (progressive enhancement)
- **Editor**: TipTap v3 (rich text editing)
- **Build**: ESBuild for JS bundling, Tailwind CLI for CSS

### Key Features
- User authentication (register/login/logout)
- Post management (create/edit/delete) with slug generation
- Comment system with AJAX enhancement
- WYSIWYG editing with TipTap
- Responsive design with dark mode support

## Code Guidelines

### Go Backend
```go
// Use repository pattern for data access
type PostRepository struct {
    db squirrel.StatementBuilderType
}

// Always use context for database operations
func (repo *PostRepository) GetByID(ctx context.Context, id string) (*Post, error) {
    // Implementation
}

// Detect AJAX requests and provide appropriate responses
if r.Header.Get("X-Alpine-Request") == "true" {
    // Return partial HTML for AJAX
    // Always include proper error handling
}
```

### Frontend JavaScript
```javascript
// Alpine.js initialization (always at the end)
window.Alpine = Alpine
Alpine.plugin(ajax)
Alpine.start()

// Use Alpine AJAX attributes for forms
// x-target="element-id" - target for updates
// x-target.422="form-id" - target for validation errors
// x-autofocus - restore focus after AJAX
```

### HTML Templates
```html
<!-- Always provide fallback form actions -->
<form method="POST" action="/comments" x-target="comments-list comment-form">
    <!-- Form works without JS, enhanced with Alpine AJAX -->
</form>

<!-- Use semantic HTML structure -->
<main class="gap-4">
    <article><!-- Post content --></article>
    <section id="comments"><!-- Comments section --></section>
</main>
```

## File Structure Patterns

### Templates
- `page-header.gohtml` / `page-footer.gohtml` - HTML document structure
- `header.gohtml` / `footer.gohtml` - Site navigation/branding
- `*-page.gohtml` - Full page templates
- `*-list.gohtml` / `*-form.gohtml` - Partial templates for AJAX

### Static Assets
- `static/` - Compiled assets (CSS, JS, fonts)
- `templates/` - Go templates
- `migrations/` - Database schema versions

## Development Practices

### Database
- Use Squirrel query builder for type safety
- Always handle transactions properly
- Implement proper error logging with context
- Use UUIDs for primary keys

### Security
- Sanitize all HTML input with bluemonday
- Use CSRF tokens on all forms
- Hash passwords with bcrypt
- Validate and escape user input

### Performance
- Single binary deployment
- Embedded static assets
- Efficient SQL queries with proper indexing
- Minimal JavaScript bundle size

## Common Patterns

### Slug Generation
```go
// Generate unique slugs with numeric suffixes if needed
// "my-post" -> "my-post-2" if "my-post" exists
func (h *Handler) generateUniqueSlug(ctx context.Context, baseSlug string) (string, error)
```

### AJAX Response Handling
```go
// Check for AJAX requests and return appropriate content
if r.Header.Get("X-Alpine-Request") == "true" {
    // Return combined templates for multiple targets
    var buf bytes.Buffer
    h.tmpl.ExecuteTemplate(&buf, "list-template.gohtml", data)
    h.tmpl.ExecuteTemplate(&buf, "form-template.gohtml", data)
    w.Write(buf.Bytes())
    return
}
// Fallback to redirect for non-AJAX
http.Redirect(w, r, "/success-page", http.StatusSeeOther)
```

### Form Validation
- Server-side validation is mandatory
- Client-side validation is enhancement only
- Return 422 status for validation errors
- Use Alpine AJAX `x-target.422` for error display

## Build System
- `make build` - Build Go binary
- `make npm-build` - Build JS/CSS assets
- Uses ESBuild for JavaScript bundling
- Uses Tailwind CLI for CSS compilation

## Content Management Philosophy
- No separate admin panel - edit in place
- WordPress-like content flow
- Rich text editing with TipTap
- Automatic excerpt generation
- SEO-friendly URLs with proper slugs

## Error Handling
- Always log errors with context
- Provide user-friendly error messages
- Graceful degradation for JS failures
- Proper HTTP status codes

## When Adding New Features
1. **Plan for no-JS first** - ensure basic functionality works
2. **Add minimal dependencies** - prefer standard library
3. **Follow existing patterns** - repository pattern, template structure
4. **Test both modes** - with and without JavaScript
5. **Consider security** - sanitization, validation, CSRF
6. **Maintain performance** - efficient queries, minimal assets

## Code Quality Guidelines

### Code Comments
- **Avoid trivial comments** - code should be self-explanatory
- Only add comments for complex business logic or non-obvious decisions
- Prefer clear variable and function names over comments
- Document public APIs and exported functions

```go
// Good - explains WHY, not WHAT
// Generate unique slug by appending incremental numbers if base slug exists
func (h *Handler) generateUniqueSlug(ctx context.Context, baseSlug string) (string, error)

// Bad - trivial comment
// Create a new user
user := &User{Name: name}
```

### Documentation-Driven Development
- **Always read related documentation first** before implementing features
- When working with external libraries (Alpine.js, TipTap, etc.), consult official docs
- If documentation is unclear or missing, ask the user to provide relevant documentation
- Reference specific documentation sections in code when implementing complex features

```go
// Example: When implementing Alpine AJAX features
// Refer to: https://alpine-ajax.js.org/reference/#x-target
// Before writing AJAX handling code
```

### Code Readability
- Use descriptive variable and function names
- Keep functions focused on single responsibilities
- Prefer explicit error handling over silent failures
- Write code that tells a story without needing comments

This system prioritizes simplicity, security, and user experience while maintaining the flexibility to grow as needed.

# NGINX UI - Claude Code Guidelines

This project is a web-based NGINX management interface built with Go backend and Vue.js frontend.

## Package Manager
- **Use pnpm exclusively** for all frontend package management operations
- Commands: `pnpm install`, `pnpm run dev`, `pnpm typecheck`

## Backend (Go) Development

### Technologies & Frameworks
- **Go** with Gin web framework
- **GORM** for database operations  
- **Gen** for query simplification
- **Cosy** framework (https://cosy.uozi.org/)

### Code Organization
- **API Controllers**: Implement in `api/$module_name/` directory
- **Database Models**: Define in `model/` folder
- **Business Logic**: Place complex logic and error handling in `internal/$module_name/`
- **Routing**: Register routes in `router/` directory
- **Configuration**: Manage settings in `settings/` directory

### Development Guidelines
- Write concise, maintainable Go code with clear examples
- Run `gofmt`/`goimports` before committing backend changes
- Use Gen to streamline database queries and reduce boilerplate
- Follow Cosy Error Handler best practices for error management
- Implement standardized CRUD operations using Cosy framework
- Apply efficient database pagination for large datasets
- Validate changes with `go test ./... -race -cover` before pushing
- Keep files modular and well-organized by functionality
- **All comments and documentation must be in English**

## Frontend (Vue.js) Development

### Technology Stack
- **TypeScript** for all code
- **Vue 3** with Composition API
- **Vite** build tool
- **Vue Router** for routing
- **Pinia** for state management
- **VueUse** for utilities
- **Ant Design Vue** for UI components
- **UnoCSS** for styling

### Code Standards
- Use functional and declarative programming patterns (avoid classes)
- Prefer interfaces over types for better extendability
- Use descriptive variable names with auxiliary verbs (e.g., `isLoading`, `hasError`)
- Always use Vue Composition API with `<script setup>` syntax
- Leverage Vue 3.4+ features: `defineModel()`, `useTemplateRef()`, v-bind shorthand

### File Organization
- **Components**: Use CamelCase naming in `src/components/` (e.g., `ChatGPT/ChatGPT.vue`)
- **Views**: Use lowercase with underscores for folders, CamelCase for components (e.g., `src/views/system/About.vue`)
- **API & Types**: Define in `app/src/api/`
- **Exports**: Favor named exports for functions

### UI & Styling
- Use Ant Design Vue components and UnoCSS for styling
- Implement responsive design with mobile-first approach
- Use Antdv Flex layout for responsive layouts

### Performance Optimization
- Leverage VueUse functions for enhanced reactivity
- Wrap async components in Suspense with fallback UI
- Use dynamic loading for non-critical components
- Optimize images: WebP format, size data, lazy loading
- Implement code splitting and chunking strategy in Vite build
- Optimize Web Vitals (LCP, CLS, FID)

### Code Quality
- **Always use ESLint MCP after generating frontend code** to ensure code quality and consistency
- Run `pnpm lint`, `pnpm lint:fix`, and `pnpm typecheck` to keep style and typings aligned

## Development Commands
- **Frontend**: `pnpm run dev`, `pnpm lint`, `pnpm typecheck`, `pnpm run build`
- **Backend**: `go generate ./...`, `go build ./...`, run `go test ./... -race -cover`; for release artifacts reuse the README command with `-tags=jsoniter -ldflags "$LD_FLAGS ..."`.
- **Demo stack**: `docker-compose -f docker-compose-demo.yml up` to bootstrap the sample environment

## Language Requirements
- **All code comments, documentation, and communication must be in English**
- Maintain consistency and accessibility across the codebase
- 优先使用 context7 mcp 搜索文档
- 生成 find 命令的时候应该排除掉 ./.go/ 这个文件夹，因为那里面是 devcontainer 的依赖
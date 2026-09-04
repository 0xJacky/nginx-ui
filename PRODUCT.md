# Product

## Register

product

## Users

Nginx UI serves system administrators and DevOps engineers who manage NGINX configuration, certificates, logs, runtime health, and multi-node deployments. They use the product while operating live infrastructure and need to understand system state quickly without guessing what background work is doing.

## Product Purpose

Nginx UI provides a practical web interface for configuring and operating NGINX across one or more servers. Success means an operator can make a change, understand its effect, recover from failures, and verify the resulting runtime state with confidence.

## Brand Personality

Reliable, clear, and restrained. The interface should communicate operational truth directly, use familiar administration patterns, and reserve emphasis for actions or states that need attention.

## Anti-references

Avoid decorative consumer-SaaS styling, opaque background activity, status displays that rely on color alone, excessive motion, and optimistic success states that are not backed by runtime evidence.

## Design Principles

1. Show the current state and the transition in progress separately.
2. Explain failures with an actionable next step while preserving technical detail for diagnosis.
3. Prefer familiar, compact administration controls over custom interaction patterns.
4. Keep destructive and security-sensitive operations explicit and verifiable.
5. Preserve the existing Ant Design Vue visual language across light and dark themes.

## Accessibility & Inclusion

Target WCAG AA contrast and interaction behavior. All important state must be conveyed with text or icons in addition to color. Interactive elements must remain keyboard accessible, focus-visible, usable in dark mode, and compatible with reduced-motion preferences.

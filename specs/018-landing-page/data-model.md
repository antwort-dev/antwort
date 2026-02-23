# Content Model: Landing Page & Documentation Site

**Feature**: 018-landing-page
**Date**: 2026-02-23

No database entities. The content model describes how landing page content maps to AstroWind widget props.

## Landing Page Widget Composition

The `index.astro` page composes AstroWind widgets in this order:

### 1. Hero Widget

```typescript
Hero({
  actions: [
    { variant: 'primary', text: 'Get Started', href: '/docs/...' },
    { text: 'View on GitHub', href: 'https://github.com/rhuss/antwort', target: '_blank' }
  ],
  title: 'The server-side agentic framework.',
  subtitle: 'The open-source OpenResponses API implementation for production...',
  tagline: 'OpenResponses-compliant. Kubernetes-native.',
  // Code snippet or image in content slot
})
```

### 2. Brands Widget (Provider Logos)

```typescript
Brands({
  title: 'Works with',
  images: [
    { src: 'vllm-logo', alt: 'vLLM' },
    { src: 'litellm-logo', alt: 'LiteLLM' },
    // ...
  ]
})
```

### 3. Features Widget (Value Pillars)

```typescript
Features({
  id: 'pillars',
  tagline: 'Why Antwort',
  title: 'Built for production AI agents',
  items: [
    { title: 'OpenResponses API', description: '...', icon: 'tabler:api' },
    { title: 'Secure by Default', description: '...', icon: 'tabler:shield-lock' },
    { title: 'Kubernetes Native', description: '...', icon: 'tabler:brand-kubernetes' },
  ]
})
```

### 4. Features Widget (Feature Grid)

```typescript
Features({
  id: 'features',
  tagline: 'Features',
  title: 'Everything you need for agentic AI',
  items: [
    // 9 implemented features
    { title: 'Agentic Loop', description: '...', icon: 'tabler:refresh' },
    // ...
    // 6 coming-soon features with callToAction or tag
    { title: 'Sandbox Execution', description: '...', icon: 'tabler:box', callToAction: { text: 'Coming Soon' } },
  ]
})
```

### 5. Content Widget (Architecture)

```typescript
Content({
  tagline: 'Architecture',
  title: 'How it works',
  // Architecture diagram as image
  image: { src: architectureSvg, alt: 'Antwort Architecture' },
  items: [
    { title: 'Gateway', description: 'Auth, routing, rate limiting' },
    { title: 'Engine', description: 'Agentic loop with concurrent tool execution' },
    { title: 'Providers', description: 'vLLM, LiteLLM, Ollama, cloud APIs' },
    { title: 'Sandbox', description: 'Kubernetes pods with gVisor isolation' },
  ]
})
```

### 6. Comparison Table (Custom HTML)

Inline HTML with Tailwind classes in `index.astro`. Five columns, three row groups, visual indicators. Not a widget since AstroWind has no comparison table component.

### 7. Steps Widget (Quickstart)

```typescript
Steps({
  tagline: 'Get Started',
  title: 'Deploy in minutes',
  items: [
    { title: 'Deploy', description: 'kubectl apply -k ...', icon: 'tabler:rocket' },
    { title: 'Send a request', description: 'curl -X POST ...', icon: 'tabler:send' },
    { title: 'Go agentic', description: 'Add tools and let the agent work', icon: 'tabler:robot' },
  ]
})
```

### 8. Steps or Timeline (Roadmap)

```typescript
Steps({
  tagline: 'Roadmap',
  title: 'What we are building',
  items: [
    { title: 'Sandbox Executor', description: '...', icon: 'tabler:box' },
    { title: 'Agent Profiles', description: '...', icon: 'tabler:user-cog' },
    // ...6 phases
  ]
})
```

### 9. CallToAction Widget

```typescript
CallToAction({
  title: 'Ready to deploy agentic AI?',
  subtitle: 'Open source. Apache 2.0.',
  actions: [
    { variant: 'primary', text: 'Get Started', href: '/docs/...' },
    { text: 'View on GitHub', href: 'https://github.com/rhuss/antwort' },
  ]
})
```

## Documentation Structure (Antora)

Same as previous plan. 11 AsciiDoc pages under `docs/modules/ROOT/pages/` in the main repo. Already created and verified.

## Logo Assets

Same as previous plan. SVG logo mark (A! in circle), wordmark, and favicon. Already created. Will be moved into the Astro `src/assets/images/` directory.

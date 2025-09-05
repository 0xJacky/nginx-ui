# Translation Development Guide

## Weblate Translation Platform

We are excited to announce the public beta of our Weblate translation platform for Nginx UI! This is a significant milestone in our mission to make Nginx UI accessible to users worldwide through comprehensive multilingual support.

**Quick Start:** Visit [Weblate Platform](https://weblate.nginxui.com) to begin translating.

### About Weblate

Weblate is a robust, user-friendly translation management platform that enables community members to contribute translations efficiently. The platform streamlines the localization process with an intuitive interface suitable for contributors of all experience levels.

### How to Contribute

We welcome all community members interested in improving global accessibility for Nginx UI. Your linguistic expertise, whether as a native speaker or proficient user, is valuable to the project.

To begin contributing:
1. Visit [https://weblate.nginxui.com](https://weblate.nginxui.com)
2. Create an account or log in with GitHub
3. Select your target language
4. Start translating available strings

Your contributions directly help expand Nginx UI's reach to a global audience.

### Support and Feedback

For issues, questions, or enhancement suggestions regarding the translation platform, please submit feedback through our GitHub issues or community channels.

## Local Translation Environment

For developers working on translations locally, we recommend using the i18n-gettext VSCode extension.

**Extension Details:**
- Documentation: [GitHub Repository](https://github.com/akinoccc/i18n-gettext)
- VSCode Marketplace: [i18n-gettext Extension](https://marketplace.visualstudio.com/items?itemName=akino.i18n-gettext)

This extension offers AI-powered translation capabilities with high-quality output and support for additional scoring models to validate translations.

## Translation Workflow for Developers

After making code changes that affect translatable content, run these commands to update translation templates:

```bash
# Generate Go i18n files
go generate

# Extract translatable strings from the frontend
cd app
pnpm gettext:extract
```

This process ensures all new translatable content is properly added to the translation system.
# 🤝 Contributing

We welcome contributions! Here's how you can help:

## Reporting Issues

1. Check if the issue already exists in GitHub Issues
1. If not, create a new issue with:
    * Clear title and description
    * Steps to reproduce
    * Expected vs actual behavior
    * Screenshots (if applicable)

## Submitting Pull Requests

1. Fork the repository
1. Create a feature branch:
    ```shell
    git checkout -b feature/amazing-feature
    ```
1. Make your changes following the coding style
1. Test thoroughly:
    ```shell
    bash
    npm run build
    npm run preview
    ```
1. Commit with clear messages:
    ```shell
    git commit -m "feat: add amazing feature"
    ```
1. Push to your fork:
    ```shell
    git push origin feature/amazing-feature
    ```
1. Open a Pull Request with:
    * Clear description of changes
    * Link to related issue (if applicable)
    * Screenshots/demos (if UI changes)

## Development Guidelines

### Code Style

* Use TypeScript with strict typing
* Follow existing code formatting
* Use ESLint and Prettier (run npm run lint)
* Write meaningful comments for complex logic

### Component Structure

```
src/
├── components/        # React components
├── services/          # Business logic
├── types/             # TypeScript definitions
├── locales/           # i18n translations
└── utils/             # Helper functions
```

### Adding Translations
1. Add keys to src/locales/en/translation.json
1. Add corresponding translations to src/locales/fr/translation.json
1. Use in components: const { t } = useTranslation();

### Adding Features
* Keep components small and focused
* Use hooks for state management
* Maintain accessibility (ARIA labels, keyboard navigation)
* Test with both GitHub and GitLab sources

### Ideas for Contribution
* 🌟 Add support for more Git providers (Bitbucket, Gitea)
* 📊 Export graph as PNG/SVG
* 🔍 Search and filter nodes
* 📈 Performance improvements for large repositories
* 🧪 Unit and integration tests
* 📚 Improve documentation

# Kustomize Visualizer

![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)
![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)
![AI-Assisted](https://img.shields.io/badge/AI--Assisted-Perplexity-blueviolet.svg)

A modern web application to visualize and explore Kustomize overlay structures in GitOps-managed environments.

## 📖 Overview

### Why Kustomize Visualizer?

In GitOps environments, **Kustomize** is a powerful tool for managing Kubernetes manifests across multiple environments (dev, staging, production) using overlays and bases. However, as projects grow in complexity, understanding the relationships between bases, overlays, components, resources, and patches becomes increasingly challenging.

**Kustomize Visualizer** addresses this problem by providing:

- **🔍 Visual Graph Representation**: Interactive graph showing the complete dependency tree of your Kustomize structure
- **🎯 Quick Navigation**: Instantly understand which overlays depend on which bases
- **🔗 Relationship Mapping**: Visualize resources, components, patches, and their interconnections
- **🌐 GitOps Integration**: Load configurations directly from GitHub or GitLab repositories
- **📊 Real-time Analysis**: Parse and display kustomization.yaml files with full dependency resolution

### Use Cases

- **DevOps Teams**: Understand complex multi-environment Kustomize structures at a glance
- **Code Reviews**: Quickly verify overlay relationships and patch applications
- **Documentation**: Generate visual documentation of your GitOps structure
- **Debugging**: Identify missing dependencies or circular references
- **Onboarding**: Help new team members understand the project architecture

---

## ✨ Features

### Current Capabilities

#### 📂 Multiple Source Support
- **GitHub**: Load repositories directly using URLs (with optional token authentication)
- **GitLab**: Full support for GitLab repositories (with optional token authentication)
- **Local Files**: Browse and scan local directories (web browser File System API)

#### 🎨 Interactive Visualization
- **Graph Canvas**: Cytoscape.js-powered interactive graph with zoom, pan, and drag
- **Node Types**:
  - 🔵 **Resources**: Kubernetes manifest files
  - 🟠 **Components**: Reusable Kustomize components
  - 🟢 **Bases**: Foundation configurations
  - 🟡 **Overlays**: Environment-specific customizations
- **Edge Types**: Visual distinction between resources, bases, components, and patches
- **Legend**: Clear indication of node and edge meanings

#### 📋 Detailed Panel
- Click any node to view detailed information:
  - Path and type
  - Kustomization content (resources, bases, components, patches)
  - Full YAML structure
- Collapsible sidebar for distraction-free visualization

#### 🌍 Internationalization
- **English** and **French** support
- Easy language switching
- Extensible translation system (i18next)

#### 🔐 Security
- **Token Storage**: Encrypted storage of GitHub/GitLab tokens using Web Crypto API
- **Settings Modal**: Secure token management with visibility toggle
- **IndexedDB**: Persistent, encrypted local storage

---

## 🚀 Getting Started

### Using Docker/Podman (Recommended)

#### Build the Container

```bash
# Clone the repository
git clone https://github.com/cjeanner/kustomize-visualizer.git
cd kustomize-visualizer

# Build the image with Podman
podman build -t kustomize-visualizer:latest -f Containerfile .

# Or with Docker
docker build -t kustomize-visualizer:latest -f Containerfile .
```

### Run the Container
```bash
# With Podman
podman run -d -p 8080:80 --name kustomize-viz kustomize-visualizer:latest

# Or with Docker
docker run -d -p 8080:80 --name kustomize-viz kustomize-visualizer:latest
```

### Access the Application
Open your browser and navigate to:
```shell
http://localhost:8080
```

### Stop and remove
```shell
# Stop
podman stop kustomize-viz

# Remove
podman rm kustomize-viz

# Remove image
podman rmi kustomize-visualizer:latest
```

## 🛠️ Development Setup
Prerequisites
* Node.js 20+ and npm
* Git

### Local Development

```shell
# Clone the repository
git clone https://github.com/cjeanner/kustomize-visualizer.git
cd kustomize-visualizer

# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview
```

### Electron Desktop App

```shell
# Start Electron in development mode
npm run electron:dev

# Build Electron app
npm run electron:build
```

## 📖 Usage Guide
**Load a Repository**

*GitHub/GitLab*
1. Enter the repository URL (e.g., https://github.com/user/repo)
1. Click "🌐 Load from Remote"
1. Optional: Configure authentication tokens in Settings (☰ menu) for private repositories

*Local Files*
1. Click "📁 Select Local Directory"
1. Choose a directory containing kustomization.yaml files
1. The app will recursively scan and visualize the structure

**Explore the Graph**
* Pan: Click and drag the background
* Zoom: Use mouse wheel or pinch gesture
* Select Node: Click any node to view details in the right sidebar
* Fit View: Use layout controls to reset the view

**Analyze Relationships**
* Blue nodes: Individual resources
* Orange nodes: Components
* Edges: Show dependencies (bases, resources, components, patches)
* Hover over nodes for tooltips


## 🤖 AI-Assisted Development

This project was developed with the assistance of Perplexity AI, leveraging
advanced language models to accelerate development and ensure code quality.

### Technical Details
* AI Platform: Perplexity
* Primary Model: Claude Sonnet 4.5 Thinking (Anthropic)
* Development Approach: Human-AI collaborative coding

### Transparency Statement

We believe in transparency regarding AI usage in software development. While AI assisted in:
* Code generation and architecture design
* Problem-solving and debugging
* Documentation writing
* Best practices implementation

All code has been:
* ✅ Reviewed and validated by human developers
* ✅ Tested for functionality and security
* ✅ Adapted to specific project requirements
* ✅ Maintained with human oversight

The use of AI tools does not diminish the quality or reliability of the
software. Instead, it demonstrates modern development practices that combine
human expertise with AI capabilities to deliver robust solutions efficiently.

## Contributing as a Human Developer

Human contributions are highly valued! Whether you're fixing bugs, adding
features, or improving documentation, your expertise and creativity are
essential to the project's growth. AI assistance complements—but does not
replace—human ingenuity and domain knowledge.

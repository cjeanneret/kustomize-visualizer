// --- Token storage encryption (Web Crypto API) ---
const TOKEN_STORAGE_KEYS = { github: 'github_token', gitlab: 'gitlab_token' };
const STORAGE_ENABLED_KEY = 'tokens_storage_enabled';
const PBKDF2_ITERATIONS = 100000;
const AES_GCM_IV_LENGTH = 12;
const SALT_LENGTH = 16;

/**
 * Derives an AES-GCM key from a password string and salt using PBKDF2.
 * @param {string} password - Source string (e.g. origin + app id)
 * @param {Uint8Array} salt - Random salt
 * @returns {Promise<CryptoKey>}
 */
async function deriveKey(password, salt) {
    const enc = new TextEncoder();
    const keyMaterial = await crypto.subtle.importKey(
        'raw',
        enc.encode(password),
        'PBKDF2',
        false,
        ['deriveBits', 'deriveKey']
    );
    return crypto.subtle.deriveKey(
        {
            name: 'PBKDF2',
            salt,
            iterations: PBKDF2_ITERATIONS,
            hash: 'SHA-256',
        },
        keyMaterial,
        { name: 'AES-GCM', length: 256 },
        false,
        ['encrypt', 'decrypt']
    );
}

/**
 * Encrypts a plaintext string for storage. Uses AES-GCM with a key derived
 * from origin + app id and a random salt (stored with the payload).
 * @param {string} plaintext
 * @returns {Promise<string>} Base64-encoded JSON: { s: saltB64, i: ivB64, c: ciphertextB64 }
 */
async function encryptToken(plaintext) {
    const salt = crypto.getRandomValues(new Uint8Array(SALT_LENGTH));
    const iv = crypto.getRandomValues(new Uint8Array(AES_GCM_IV_LENGTH));
    const password = (window.location?.origin || '') + 'kustomap-token-v1';
    const key = await deriveKey(password, salt);

    const enc = new TextEncoder();
    const ciphertext = await crypto.subtle.encrypt(
        { name: 'AES-GCM', iv },
        key,
        enc.encode(plaintext)
    );

    const payload = {
        s: btoa(String.fromCharCode(...salt)),
        i: btoa(String.fromCharCode(...iv)),
        c: btoa(String.fromCharCode(...new Uint8Array(ciphertext))),
    };
    return btoa(unescape(encodeURIComponent(JSON.stringify(payload))));
}

/**
 * Decrypts a payload produced by encryptToken.
 * @param {string} encoded - Base64-encoded JSON payload
 * @returns {Promise<string>} Decrypted plaintext
 */
async function decryptToken(encoded) {
    try {
        const jsonStr = decodeURIComponent(escape(atob(encoded)));
        const payload = JSON.parse(jsonStr);
        const salt = Uint8Array.from(atob(payload.s), (c) => c.charCodeAt(0));
        const iv = Uint8Array.from(atob(payload.i), (c) => c.charCodeAt(0));
        const ciphertext = Uint8Array.from(atob(payload.c), (c) => c.charCodeAt(0));
        const password = (window.location?.origin || '') + 'kustomap-token-v1';
        const key = await deriveKey(password, salt);

        const decrypted = await crypto.subtle.decrypt(
            { name: 'AES-GCM', iv },
            key,
            ciphertext
        );
        return new TextDecoder().decode(decrypted);
    } catch (e) {
        return null;
    }
}

/**
 * Detects if a localStorage value is our encrypted format (v1) or legacy plain text.
 * @param {string} value
 * @returns {boolean}
 */
function isEncryptedPayload(value) {
    if (!value || typeof value !== 'string') return false;
    try {
        const decoded = atob(value);
        const o = JSON.parse(decoded);
        return o && typeof o.s === 'string' && typeof o.i === 'string' && typeof o.c === 'string';
    } catch (_) {
        return false;
    }
}

// renderGraph function with full-window optimization
function renderGraph(container, graphData, onNodeClick) {
    const cy = window.cytoscape({
        container: container,
        elements: graphData.elements,
        style: [
            {
                selector: 'node',
                style: {
                    'label': 'data(label)',
                    'text-valign': 'center',
                    'text-halign': 'center',
                    'width': 280,
                    'height': 44,
                    'font-size': '12px',
                    'text-wrap': 'wrap',
                    'text-max-width': '260px',
                    'border-width': 2,
                    'border-color': '#333',
                    'shape': 'roundrectangle'
                }
            },
            {
                selector: 'node[type="base"]',
                style: {
                    'background-color': '#2ecc71'
                }
            },
            {
                selector: 'node[type="overlay"]',
                style: {
                    'background-color': '#3498db'
                }
            },
            {
                selector: 'node[type="component"]',
                style: {
                    'background-color': '#9b59b6'
                }
            },
            {
                selector: 'node[type="resource"]',
                style: {
                    'background-color': '#3498db'
                }
            },
            {
                selector: 'node[type="error"]',
                style: {
                    'background-color': '#e74c3c',
                    'border-color': '#c0392b',
                    'border-width': 3,
                    'color': 'white'
                }
            },
            {
                selector: 'edge',
                style: {
                    'width': 2,
                    'line-color': '#95a5a6',
                    'target-arrow-color': '#95a5a6',
                    'target-arrow-shape': 'triangle',
                    'curve-style': 'bezier',
                    'arrow-scale': 1.2
                }
            },
            {
                selector: 'node:selected',
                style: {
                    'border-width': 4,
                    'border-color': '#e67e22',
                    'background-color': '#f39c12'
                }
            }
        ],
        layout: {
            name: 'dagre',
            rankDir: 'TB',
            nodeSep: 50,
            rankSep: 100,
            padding: 30
        },
        minZoom: 0.1,
        maxZoom: 3,
        wheelSensitivity: 0.2
    });

    cy.on('tap', 'node', function(evt) {
        const node = evt.target;
        if (onNodeClick) {
            onNodeClick(node.data());
        }
    });

    // Resize handler for full-window responsiveness
    window.addEventListener('resize', () => {
        cy.resize();
        //cy.fit();
    });

    return cy;
}

// Main App
class App {
    constructor() {
        this.form = document.getElementById('analyze-form');
        this.formSection = document.getElementById('form-section');
        this.graphSection = document.getElementById('graph-section');
        this.graphContainer = document.getElementById('graph-container');
        this.errorDiv = document.getElementById('error');
        this.errorMessage = document.getElementById('error-message');
        this.graphErrorDiv = document.getElementById('graph-error');
        this.graphErrorMessage = document.getElementById('graph-error-message');
        this.analyzeBtn = document.getElementById('analyze-btn');
        this.backBtn = document.getElementById('back-btn');
        this.fitBtn = document.getElementById('fit-btn');
        this.centerBtn = document.getElementById('center-btn');
        this.exportPngBtn = document.getElementById('export-png-btn');
        this.exportSvgBtn = document.getElementById('export-svg-btn');
        this.exportMermaidBtn = document.getElementById('export-mermaid-btn');
        this.sidebar = document.getElementById('sidebar');
        this.sidebarContent = document.getElementById('sidebar-content');
        this.closeSidebar = document.getElementById('close-sidebar');

        this.currentGraphId = null;
        this.currentGraphData = null;
        this.cy = null;

        this.init();
    }

    init() {
        this.form.addEventListener('submit', (e) => this.handleSubmit(e));
        this.backBtn.addEventListener('click', () => this.showForm());
        this.closeSidebar.addEventListener('click', () => this.hideSidebar());
        this.fitBtn.addEventListener('click', () => this.fitGraph());
        this.centerBtn.addEventListener('click', () => this.centerGraph());
        this.exportPngBtn.addEventListener('click', () => this.exportPNG());
        this.exportSvgBtn.addEventListener('click', () => this.exportSVG());
        this.exportMermaidBtn.addEventListener('click', () => this.exportMermaid());

        this.loadTokensFromStorage();
        this.setupTokenPersistence();
        this.setupTokenVisibilityToggles();
        document.getElementById('clear-tokens-btn')?.addEventListener('click', () => this.clearStoredTokens());
    }

    setupTokenVisibilityToggles() {
        const pairs = [
            { inputId: 'github-token', toggleId: 'github-token-toggle' },
            { inputId: 'gitlab-token', toggleId: 'gitlab-token-toggle' },
        ];
        pairs.forEach(({ inputId, toggleId }) => {
            const input = document.getElementById(inputId);
            const toggle = document.getElementById(toggleId);
            if (!input || !toggle) return;
            toggle.addEventListener('click', () => {
                const isPassword = input.type === 'password';
                input.type = isPassword ? 'text' : 'password';
                toggle.classList.toggle('revealed', isPassword);
                toggle.title = isPassword ? 'Hide token' : 'Show token';
                toggle.setAttribute('aria-label', isPassword ? 'Hide token' : 'Show token');
            });
        });
    }

    clearStoredTokens() {
        localStorage.removeItem(TOKEN_STORAGE_KEYS.github);
        localStorage.removeItem(TOKEN_STORAGE_KEYS.gitlab);
        const storeCheckbox = document.getElementById('store-tokens');
        if (storeCheckbox) storeCheckbox.checked = false;
        localStorage.setItem(STORAGE_ENABLED_KEY, '0');
        const githubEl = document.getElementById('github-token');
        const gitlabEl = document.getElementById('gitlab-token');
        if (githubEl) githubEl.value = '';
        if (gitlabEl) gitlabEl.value = '';
    }

    async loadTokensFromStorage() {
        const githubEl = document.getElementById('github-token');
        const gitlabEl = document.getElementById('gitlab-token');
        const storeCheckbox = document.getElementById('store-tokens');

        const hasStoredTokens = localStorage.getItem(TOKEN_STORAGE_KEYS.github) || localStorage.getItem(TOKEN_STORAGE_KEYS.gitlab);
        const preference = localStorage.getItem(STORAGE_ENABLED_KEY);
        const enabled = preference === '1' || (preference === null && hasStoredTokens);
        if (storeCheckbox) storeCheckbox.checked = enabled;
        if (enabled && !preference) localStorage.setItem(STORAGE_ENABLED_KEY, '1');

        if (!enabled) return;

        for (const [name, storageKey] of Object.entries(TOKEN_STORAGE_KEYS)) {
            const el = name === 'github' ? githubEl : gitlabEl;
            const raw = localStorage.getItem(storageKey);
            if (!raw) continue;

            let plain;
            if (isEncryptedPayload(raw)) {
                plain = await decryptToken(raw);
            } else {
                plain = raw;
                try {
                    const encrypted = await encryptToken(plain);
                    localStorage.setItem(storageKey, encrypted);
                } catch (_) {
                    /* keep plain if encryption fails */
                }
            }
            if (plain) el.value = plain;
        }
    }

    setupTokenPersistence() {
        const storeCheckbox = document.getElementById('store-tokens');

        document.getElementById('github-token').addEventListener('blur', async (e) => {
            const value = e.target.value?.trim();
            if (!storeCheckbox?.checked) return;
            if (value) {
                try {
                    const encrypted = await encryptToken(value);
                    localStorage.setItem(TOKEN_STORAGE_KEYS.github, encrypted);
                } catch (_) {
                    localStorage.setItem(TOKEN_STORAGE_KEYS.github, value);
                }
            } else {
                localStorage.removeItem(TOKEN_STORAGE_KEYS.github);
            }
        });

        document.getElementById('gitlab-token').addEventListener('blur', async (e) => {
            const value = e.target.value?.trim();
            if (!storeCheckbox?.checked) return;
            if (value) {
                try {
                    const encrypted = await encryptToken(value);
                    localStorage.setItem(TOKEN_STORAGE_KEYS.gitlab, encrypted);
                } catch (_) {
                    localStorage.setItem(TOKEN_STORAGE_KEYS.gitlab, value);
                }
            } else {
                localStorage.removeItem(TOKEN_STORAGE_KEYS.gitlab);
            }
        });

        if (storeCheckbox) {
            storeCheckbox.addEventListener('change', () => {
                if (storeCheckbox.checked) {
                    localStorage.setItem(STORAGE_ENABLED_KEY, '1');
                } else {
                    localStorage.setItem(STORAGE_ENABLED_KEY, '0');
                    localStorage.removeItem(TOKEN_STORAGE_KEYS.github);
                    localStorage.removeItem(TOKEN_STORAGE_KEYS.gitlab);
                }
            });
        }
    }

    async handleSubmit(e) {
        e.preventDefault();

        const formData = new FormData(this.form);
        const url = formData.get('url');
        const github_token = formData.get('github_token');
        const gitlab_token = formData.get('gitlab_token');

        this.hideError();
        this.setLoading(true);

        try {
            const response = await fetch('/api/v1/analyze', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    url,
                    github_token,
                    gitlab_token
                }),
            });

            const data = await response.json();

            if (!response.ok || data.status === 'error') {
                throw new Error(data.message || 'Analysis failed');
            }

            this.currentGraphId = data.id;
            await this.loadAndDisplayGraph(data.id);
        } catch (error) {
            this.showError(error.message);
        } finally {
            this.setLoading(false);
        }
    }

    async loadAndDisplayGraph(graphId) {
        try {
            const response = await fetch(`/api/v1/graph/${graphId}`);
            if (!response.ok) {
                throw new Error('Failed to load graph');
            }

            const graphData = await response.json();
            this.currentGraphData = graphData;
            this.showGraph(graphData);
        } catch (error) {
            this.showError(error.message);
        }
    }

    showGraph(graphData) {
        this.formSection.classList.add('hidden');
        this.graphSection.classList.remove('hidden');

        try {
            const container = document.getElementById('cy');
            this.cy = renderGraph(container, graphData, (nodeData) => {
                this.showNodeDetails(nodeData);
            });
        } catch (error) {
            this.showGraphError(error.message);
        }
    }

    showGraphError(message) {
        this.graphErrorMessage.textContent = message;
        this.graphErrorDiv.classList.remove('hidden');
    }

    hideGraphError() {
        this.graphErrorDiv.classList.add('hidden');
    }

    showForm() {
        this.graphSection.classList.add('hidden');
        this.formSection.classList.remove('hidden');
        this.hideSidebar();
        this.hideGraphError();

        if (this.cy) {
            this.cy.destroy();
            this.cy = null;
        }
    }

    async showNodeDetails(nodeData) {
        this.sidebar.classList.remove('hidden');
        this.graphContainer.classList.add('sidebar-open');
        if (this.cy) this.cy.resize();

        // Afficher un état de chargement
        this.sidebarContent.innerHTML = `
        <h2>${nodeData.label || nodeData.id}</h2>
        <p>Loading details...</p>
    `;

        try {
            // Appel API pour obtenir les détails complets
            const encodedNodeId = encodeURIComponent(nodeData.id);
            const response = await fetch(`/api/v1/node/${this.currentGraphId}/${encodedNodeId}`);

            if (!response.ok) {
                throw new Error('Failed to load node details');
            }

            const nodeDetails = await response.json();

            // Afficher les détails complets
            let html = `
            <h2>${nodeDetails.label || nodeDetails.id}</h2>
            <div class="node-info">
                <p><strong>ID:</strong> ${nodeDetails.id}</p>
                <p><strong>Type:</strong> <span class="badge badge-${nodeDetails.type}">${nodeDetails.type}</span></p>
                ${nodeDetails.path ? `<p><strong>Path:</strong> <code>${nodeDetails.path}</code></p>` : ''}
            </div>
        `;

            // Relations - Parents
            if (nodeDetails.parents && nodeDetails.parents.length > 0) {
                html += '<div class="relations-section">';
                html += '<h3>⬆️ Parents</h3>';
                html += '<ul class="node-list">';
                nodeDetails.parents.forEach(parentId => {
                    html += `<li><a href="#" class="node-link" data-node-id="${parentId}">${this.getNodeLabel(parentId)}</a></li>`;
                });
                html += '</ul>';
                html += '</div>';
            }

            // Relations - Children
            if (nodeDetails.children && nodeDetails.children.length > 0) {
                html += '<div class="relations-section">';
                html += '<h3>⬇️ Children</h3>';
                html += '<ul class="node-list">';
                nodeDetails.children.forEach(childId => {
                    html += `<li><a href="#" class="node-link" data-node-id="${childId}">${this.getNodeLabel(childId)}</a></li>`;
                });
                html += '</ul>';
                html += '</div>';
            }

            // Content
            if (nodeDetails.content && Object.keys(nodeDetails.content).length > 0) {
                html += '<div class="content-section">';
                html += '<h3>📄 Content</h3>';
                html += `<pre><code>${JSON.stringify(nodeDetails.content, null, 2)}</code></pre>`;
                html += '</div>';
            }

            this.sidebarContent.innerHTML = html;

            // Ajouter les event listeners pour les liens de nodes
            this.attachNodeLinkListeners();

        } catch (error) {
            this.sidebarContent.innerHTML = `
            <h2>${nodeData.label || nodeData.id}</h2>
            <div class="error-message">
                <p>⚠️ Failed to load node details: ${error.message}</p>
            </div>
        `;
        }
    }

    // Nouvelle méthode pour obtenir le label d'un node à partir de son ID
    getNodeLabel(nodeId) {
        if (!this.currentGraphData) return nodeId;

        const element = this.currentGraphData.elements.find(
            el => el.group === 'nodes' && el.data.id === nodeId
        );

        return element ? (element.data.label || nodeId) : nodeId;
    }

    // Nouvelle méthode pour gérer les clics sur les liens de nodes
    attachNodeLinkListeners() {
        const nodeLinks = this.sidebarContent.querySelectorAll('.node-link');
        nodeLinks.forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const nodeId = e.target.dataset.nodeId;

                // Sélectionner et centrer le node dans le graphe
                if (this.cy) {
                    const node = this.cy.getElementById(nodeId);
                    if (node.length > 0) {
                        // Désélectionner tous les nodes
                        this.cy.elements().unselect();
                        // Sélectionner le node cible
                        node.select();
                        // Centrer la vue sur le node
                        this.cy.animate({
                            center: { eles: node },
                            zoom: 1.5
                        }, {
                            duration: 500
                        });
                        // Afficher les détails du node
                        this.showNodeDetails(node.data());
                    }
                }
            });
        });
    }


    hideSidebar() {
        this.sidebar.classList.add('hidden');
        this.graphContainer.classList.remove('sidebar-open');
        if (this.cy) {
            this.cy.elements().unselect();
            this.cy.resize();
        }
    }

    fitGraph() {
        if (this.cy) {
            this.cy.fit();
        }
    }

    centerGraph() {
        if (this.cy) {
            this.cy.center();
        }
    }

    exportPNG() {
        if (!this.cy) return;

        const blob = this.cy.png({ full: true, scale: 1, output: 'blob' });
        if (!blob) return;
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `kustomize-graph-${this.currentGraphId || 'export'}.png`;
        link.click();
        URL.revokeObjectURL(url);
    }

    exportSVG() {
        if (!this.cy) return;

        const svgContent = this.cy.svg({ scale: 1, full: true });
        const blob = new Blob([svgContent], { type: 'image/svg+xml' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `kustomize-graph-${this.currentGraphId || 'export'}.svg`;
        link.click();
        URL.revokeObjectURL(url);
    }

    async exportMermaid() {
        if (!this.currentGraphId) return;

        try {
            const res = await fetch(`/api/v1/graph/${this.currentGraphId}?format=mermaid`);
            if (!res.ok) throw new Error(res.statusText);
            const text = await res.text();
            const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.download = `kustomize-graph-${this.currentGraphId}.mmd`;
            link.click();
            URL.revokeObjectURL(url);
        } catch (e) {
            this.showError('Failed to export Mermaid: ' + (e.message || String(e)));
        }
    }

    setLoading(loading) {
        this.analyzeBtn.disabled = loading;
        this.analyzeBtn.textContent = loading ? 'Analyzing...' : 'Analyze';
    }

    showError(message) {
        this.errorMessage.textContent = message;
        this.errorDiv.classList.remove('hidden');
    }

    hideError() {
        this.errorDiv.classList.add('hidden');
    }
}

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    new App();
});
